package cli

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/larsartmann/go-cqrs-lite/system/v4"

	"github.com/larsartmann/google-domain-detector/internal/classify"
	"github.com/larsartmann/google-domain-detector/internal/scanstore"
)

// applyDelta narrows replay results to the domains that flipped since the
// baseline run and reports the flip tallies plus any ruleset mismatch.
func applyDelta(
	ctx context.Context,
	f ReplayFlags,
	sys *system.System,
	stderr io.Writer,
	replay scanstore.Replay,
	summary *scanstore.RunSummary,
) ([]classify.Result, error) {
	from, err := replayBaseline(ctx, sys, f.Since, f.EventStore)
	if err != nil {
		return nil, err
	}

	flips := scanstore.FlipDiff(from.replay.Results, replay.Results)

	flipSet := make(map[string]bool, len(flips))

	for _, flip := range flips {
		flipSet[flip.Domain] = true
	}

	results := make([]classify.Result, 0, len(flips))

	for _, r := range replay.Results {
		if flipSet[r.Domain] {
			results = append(results, r)
		}
	}

	counts := scanstore.CountFlips(flips)
	_, _ = fmt.Fprintf(stderr, "delta vs run %s: %d flips — workspace adopted %d / left %d, gcp adopted %d / left %d\n",
		f.Since, len(flips),
		counts.WorkspaceAdopted, counts.WorkspaceLeft,
		counts.GCPAdopted, counts.GCPLeft)

	warnRulesetMismatch(stderr, from.summary(), summary)

	return results, nil
}

// baseline is the --since side of a delta replay.
type baseline struct {
	replay scanstore.Replay
}

func (b baseline) byDomain() map[string]classify.Result {
	byDomain := make(map[string]classify.Result, len(b.replay.Results))
	for _, r := range b.replay.Results {
		byDomain[r.Domain] = r
	}

	return byDomain
}

func (b baseline) summary() *scanstore.RunSummary {
	return runSummary(b.replay)
}

func replayBaseline(ctx context.Context, sys *system.System, sinceRun, eventStore string) (baseline, error) {
	replay, err := scanstore.ReplayJournal(ctx, sys, sinceRun)
	if err != nil {
		if errors.Is(err, scanstore.ErrNoRuns) {
			return baseline{}, fmt.Errorf("%s: %w", eventStore, err)
		}

		if errors.Is(err, scanstore.ErrUnknownRun) {
			return baseline{}, fmt.Errorf("--since run %q not found in %s: %w", sinceRun, eventStore, err)
		}

		return baseline{}, err
	}

	return baseline{replay: replay}, nil
}

// isFlipAgainst reports whether the observation differs from the baseline
// state for its domain; without a baseline (--since unset) everything passes.
func isFlipAgainst(fromByDomain map[string]classify.Result, to classify.Result) bool {
	if fromByDomain == nil {
		return true
	}

	from, seen := fromByDomain[to.Domain]
	if !seen {
		return false
	}

	flips := scanstore.FlipDiff([]classify.Result{from}, []classify.Result{to})

	return len(flips) == 1
}
