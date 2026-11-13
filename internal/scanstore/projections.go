package scanstore

import (
	"context"
	"fmt"
	"time"

	"github.com/larsartmann/go-cqrs-lite/projectionhost/v4"
	"github.com/larsartmann/go-cqrs-lite/system/v4"
)

// projectionCatchUpBudget bounds how long a read command waits for the
// projection workers to fold an unfoldled journal tail. Census-scale drains
// measure ~50k events/s (245k events ≈ 5s), so 60s leaves ample headroom.
const projectionCatchUpBudget = 60 * time.Second

// projectionSettlePoll is the poll interval while waiting for workers to
// settle. Small enough to add no perceptible latency on a settled store,
// large enough not to spin.
const projectionSettlePoll = 25 * time.Millisecond

// WaitProjectionsSettled blocks until every projection worker has either
// drained the journal to its head (WorkerStopped) or subscribed to live
// events (WorkerLive). Read-model queries before that point race the
// workers' staggered catch-up drain and can observe a stale or EMPTY read
// model — deterministically reproducible with `Open` + immediate `ListRuns`
// on a journal whose tail was never folded (unclean close, or any writer
// that bypassed the host). Folds live in the workers, not in dispatch.
//
// The census-critical read commands (runs, show, enrich) call this after
// Open, before their first read-model query. Recording paths must NOT:
// their workers are the ones doing the folding.
func WaitProjectionsSettled(ctx context.Context, sys *system.System) error {
	deadline := time.Now().Add(projectionCatchUpBudget)

	for {
		states := sys.WorkerStatus()
		settled := true

		for _, w := range states {
			switch w.Status {
			case projectionhost.WorkerLive, projectionhost.WorkerStopped:
				// Settled: tailing live, or drained to the head and exited.
			case projectionhost.WorkerFailed:
				return fmt.Errorf("projection %s failed while catching up (restarts %d): %s",
					w.Name, w.Restarts, w.LastError)
			default:
				// WorkerIdle: staggered start has not launched it yet.
				// WorkerRunning/WorkerDraining/WorkerBackoff: still folding.
				settled = false
			}
		}

		if settled {
			return nil
		}

		if ctx.Err() != nil {
			return fmt.Errorf("projection catch-up canceled: %w", ctx.Err())
		}

		if time.Now().After(deadline) {
			return fmt.Errorf("projections did not settle within %s: %s",
				projectionCatchUpBudget, workerStatesSummary(states))
		}

		select {
		case <-ctx.Done():
			return fmt.Errorf("projection catch-up canceled: %w", ctx.Err())
		case <-time.After(projectionSettlePoll):
		}
	}
}

func workerStatesSummary(states []projectionhost.WorkerState) string {
	summary := ""
	for _, w := range states {
		summary += fmt.Sprintf("%s=%s(processed=%d) ", w.Name, w.Status, w.Processed)
	}

	return summary
}
