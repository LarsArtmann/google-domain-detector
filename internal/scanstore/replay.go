package scanstore

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/larsartmann/go-cqrs-lite/event/v4"
	"github.com/larsartmann/go-cqrs-lite/id/v4"
	"github.com/larsartmann/go-cqrs-lite/system/v4"

	"github.com/larsartmann/google-domain-detector/internal/classify"
)

// RunSummary is one run as seen while walking the journal.
type RunSummary struct {
	//cqrs-lint:ignore(A032) journal-derived summaries are a serialized data surface; branded IDs stop at the command/stream boundary (goal-shaped-app precedent)
	RunID     string
	StartedAt time.Time
	Completed bool
	Counts    classify.Counts
}

// Replay is the rebuild-from-events result: the per-domain observations of
// one run (last observation wins) plus every run found in the journal.
type Replay struct {
	//cqrs-lint:ignore(A032) journal-derived summaries are a serialized data surface; branded IDs stop at the command/stream boundary (goal-shaped-app precedent)
	RunID   string
	Results []classify.Result
	Runs    []RunSummary
}

// ReplayJournal rebuilds a report purely from the event journal — no DNS, no
// read models. runFilter selects the run to rebuild; empty means the most
// recently started run.
func ReplayJournal(ctx context.Context, sys *system.System, runFilter string) (Replay, error) {
	store := sys.EventStore()

	events, err := readAll(ctx, store)
	if err != nil {
		return Replay{}, fmt.Errorf("read event journal: %w", err)
	}

	// First pass: run bookkeeping and the latest run identity.
	runs := map[string]*RunSummary{}

	var runOrder []string

	latest := ""

	for _, evt := range events {
		switch evt.Type() {
		case evtScanStarted:
			p, err := event.DecodePayloadAuto[ScanStartedPayload](evt)
			if err != nil {
				return Replay{}, fmt.Errorf("decode %s: %w", evtScanStarted, err)
			}

			if _, seen := runs[p.RunID]; !seen {
				runs[p.RunID] = &RunSummary{RunID: p.RunID, StartedAt: p.StartedAt}
				runOrder = append(runOrder, p.RunID)
			}

			latest = p.RunID
		case evtScanCompleted:
			p, err := event.DecodePayloadAuto[ScanCompletedPayload](evt)
			if err != nil {
				return Replay{}, fmt.Errorf("decode %s: %w", evtScanCompleted, err)
			}

			if r, ok := runs[p.RunID]; ok {
				r.Completed = true
				r.Counts = p.Counts
			}
		}
	}

	if runFilter == "" {
		runFilter = latest
	}

	out := Replay{RunID: runFilter}
	for _, runID := range runOrder {
		out.Runs = append(out.Runs, *runs[runID])
	}

	// Second pass: the run's observations, last one per domain wins, in
	// first-seen order.
	byDomain := map[string]classify.Result{}

	var domainOrder []string

	for _, evt := range events {
		if evt.Type() != evtDomainObserved {
			continue
		}

		p, err := event.DecodePayloadAuto[DomainObservedPayload](evt)
		if err != nil {
			return Replay{}, fmt.Errorf("decode %s: %w", evtDomainObserved, err)
		}

		if p.RunID != runFilter {
			continue
		}

		if _, seen := byDomain[p.Domain]; !seen {
			domainOrder = append(domainOrder, p.Domain)
		}

		byDomain[p.Domain] = classify.Result{Domain: p.Domain, Evidences: p.Evidence, Err: p.Err}
	}

	for _, name := range domainOrder {
		out.Results = append(out.Results, byDomain[name])
	}

	return out, nil
}

// replayBatchSize bounds journal-walk memory for whole-TLD stores.
const replayBatchSize = 1000

// readAll walks the journal in batches when the store supports seeking, and
// falls back to a plain journal read otherwise.
func readAll(ctx context.Context, store event.Store) ([]event.Event, error) {
	if seekable, ok := store.(event.SeekableJournal); ok {
		var out []event.Event

		var after id.EventID

		for {
			batch, err := seekable.ReadFrom(ctx, after, replayBatchSize)
			if err != nil {
				return nil, err
			}

			if len(batch) == 0 {
				return out, nil
			}

			out = append(out, batch...)
			after = batch[len(batch)-1].ID()
		}
	}

	journal, ok := store.(event.Journal)
	if !ok {
		return nil, errors.New("event store supports neither seeking nor journal reads")
	}

	return journal.ReadAll(ctx)
}
