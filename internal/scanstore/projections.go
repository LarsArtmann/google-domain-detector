package scanstore

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/larsartmann/go-cqrs-lite/projectionhost/v4"
	"github.com/larsartmann/go-cqrs-lite/system/v4"
)

var (
	// errProjectionWorkerFailed marks a worker that exhausted its restart
	// budget mid-catch-up; the read model is likely incomplete.
	errProjectionWorkerFailed = errors.New("projection worker failed while catching up")
	// errProjectionsUnsettled marks a timeout waiting for the workers to
	// drain; read-model queries would race the fold.
	errProjectionsUnsettled = errors.New("projections did not settle within the catch-up budget")
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
				return fmt.Errorf("%w: %s (restarts %d): %s",
					errProjectionWorkerFailed, w.Name, w.Restarts, w.LastError)
			case projectionhost.WorkerIdle, projectionhost.WorkerRunning,
				projectionhost.WorkerBackoff, projectionhost.WorkerDraining:
				// Idle: staggered start has not launched it yet. The rest:
				// still folding.
				settled = false
			}
		}

		if settled {
			return nil
		}

		if err := ctx.Err(); err != nil {
			return fmt.Errorf("projection catch-up canceled: %w", err)
		}

		if time.Now().After(deadline) {
			return fmt.Errorf("%w after %s: %s",
				errProjectionsUnsettled, projectionCatchUpBudget, workerStatesSummary(states))
		}

		select {
		case <-ctx.Done():
			return fmt.Errorf("projection catch-up canceled: %w", ctx.Err())
		case <-time.After(projectionSettlePoll):
		}
	}
}

func workerStatesSummary(states []projectionhost.WorkerState) string {
	var summary strings.Builder

	for _, w := range states {
		fmt.Fprintf(&summary, "%s=%s(processed=%d) ", w.Name, w.Status, w.Processed)
	}

	return summary.String()
}
