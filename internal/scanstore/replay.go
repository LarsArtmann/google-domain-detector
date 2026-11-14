package scanstore

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/larsartmann/go-cqrs-lite/event/v4"
	"github.com/larsartmann/go-cqrs-lite/id/v4"
	"github.com/larsartmann/go-cqrs-lite/system/v4"

	"github.com/larsartmann/google-domain-detector/internal/classify"
)

// RunSummary is one run as seen while walking the journal.
type RunSummary struct {
	//cqrs-lint:ignore(A032) journal-derived summaries are a serialized data surface; branded IDs stop at the command/stream boundary (goal-shaped-app precedent)
	RunID       string
	StartedAt   time.Time
	Completed   bool
	ResumedFrom string
	Wave        string
	Ruleset     classify.Ruleset
	Counts      classify.Counts
}

// Replay is the rebuild-from-events result: the per-domain observations of
// one run (last observation wins) plus every run found in the journal.
type Replay struct {
	//cqrs-lint:ignore(A032) journal-derived summaries are a serialized data surface; branded IDs stop at the command/stream boundary (goal-shaped-app precedent)
	RunID   string
	Results []classify.Result
	Runs    []RunSummary
}

// ErrNoRuns reports an event store whose journal holds no scan runs.
var ErrNoRuns = errors.New("no scan runs found in event store")

// ErrUnknownRun reports a run filter (from --run or --since) that names no
// run in the journal — the caller should surface it as a user error naming
// the filter, never as an empty report.
var ErrUnknownRun = errors.New("run not found in event store")

// ReplayJournal rebuilds a report purely from the event journal — no DNS, no
// read models — in two bounded-memory walks: run bookkeeping first, then the
// selected run's observations (last one per domain wins, first-seen order).
// runFilter selects the run to rebuild; empty means the most recently
// started run.
func ReplayJournal(ctx context.Context, sys *system.System, runFilter string) (Replay, error) {
	out, err := walkRuns(ctx, sys.EventStore(), runFilter)
	if err != nil {
		return Replay{}, err
	}

	if out.RunID == "" {
		return Replay{}, ErrNoRuns
	}

	byDomain := map[string]classify.Result{}

	var domainOrder []string

	err = walkJournal(ctx, sys.EventStore(), func(evt event.Event) error {
		if evt.Type() != evtDomainObserved {
			return nil
		}

		p, err := decodeObserved(evt)
		if err != nil {
			return fmt.Errorf("decode %s: %w", evtDomainObserved, err)
		}

		if p.RunID != out.RunID {
			return nil
		}

		if _, seen := byDomain[p.Domain]; !seen {
			domainOrder = append(domainOrder, p.Domain)
		}

		byDomain[p.Domain] = classify.Result{Domain: p.Domain, Evidences: p.Evidence, Err: p.Err, ErrKind: p.ErrKind}

		return nil
	})
	if err != nil {
		return Replay{}, fmt.Errorf("read event journal: %w", err)
	}

	for _, name := range domainOrder {
		out.Results = append(out.Results, byDomain[name])
	}

	return out, nil
}

// StreamObservations is the streaming counterpart of ReplayJournal for
// callers that render as they go: it hands observe every observation of the
// selected run in journal order, walking in batches so a whole-TLD store
// never materializes. The recorder writes one observation per domain per
// run; a journal that (manually) repeats a domain yields it twice —
// last-wins folding stays with ReplayJournal.
func StreamObservations(
	ctx context.Context,
	sys *system.System,
	runFilter string,
	observe func(classify.Result) error,
) (RunSummary, error) {
	runs, err := walkRuns(ctx, sys.EventStore(), runFilter)
	if err != nil {
		return RunSummary{}, err
	}

	if runs.RunID == "" {
		return RunSummary{}, ErrNoRuns
	}

	var selected RunSummary

	for _, r := range runs.Runs {
		if r.RunID == runs.RunID {
			selected = r

			break
		}
	}

	err = walkJournal(ctx, sys.EventStore(), func(evt event.Event) error {
		if evt.Type() != evtDomainObserved {
			return nil
		}

		p, err := decodeObserved(evt)
		if err != nil {
			return fmt.Errorf("decode %s: %w", evtDomainObserved, err)
		}

		if p.RunID != runs.RunID {
			return nil
		}

		return observe(classify.Result{Domain: p.Domain, Evidences: p.Evidence, Err: p.Err, ErrKind: p.ErrKind})
	})
	if err != nil {
		return selected, fmt.Errorf("read event journal: %w", err)
	}

	return selected, nil
}

// walkRuns is the first journal pass: run summaries in first-started order
// plus the identity of the run to rebuild (runFilter, or the latest started).
func walkRuns(ctx context.Context, store event.Store, runFilter string) (Replay, error) {
	runs := map[string]*RunSummary{}

	var runOrder []string

	latest := ""

	err := walkJournal(ctx, store, func(evt event.Event) error {
		switch evt.Type() {
		case evtScanStarted:
			payload, err := event.DecodePayloadAuto[ScanStartedPayload](evt)
			if err != nil {
				return fmt.Errorf("decode %s: %w", evtScanStarted, err)
			}

			if _, seen := runs[payload.RunID]; !seen {
				runs[payload.RunID] = &RunSummary{
					RunID:       payload.RunID,
					StartedAt:   payload.StartedAt.Time(),
					ResumedFrom: payload.ResumedFrom,
					Wave:        payload.Wave,
					Ruleset:     payload.Ruleset,
				}
				runOrder = append(runOrder, payload.RunID)
			}

			latest = payload.RunID
		case evtScanCompleted:
			p, err := event.DecodePayloadAuto[ScanCompletedPayload](evt)
			if err != nil {
				return fmt.Errorf("decode %s: %w", evtScanCompleted, err)
			}

			if r, ok := runs[p.RunID]; ok {
				r.Completed = true
				r.Counts = p.Counts
			}
		}

		return nil
	})
	if err != nil {
		return Replay{}, fmt.Errorf("read event journal: %w", err)
	}

	if runFilter == "" {
		runFilter = latest
	}

	if len(runs) == 0 {
		return Replay{}, ErrNoRuns
	}

	if _, known := runs[runFilter]; !known {
		return Replay{}, fmt.Errorf("%w: %q", ErrUnknownRun, runFilter)
	}

	out := Replay{RunID: runFilter}
	for _, runID := range runOrder {
		out.Runs = append(out.Runs, *runs[runID])
	}

	return out, nil
}

// RunSummaries reads every run summary from the journal alone, in
// first-started order. It is the journal-only counterpart of ListRuns:
// Boot-composed reads (replay, the supersession audit) resolve runs and
// waves through it instead of the projection read models, which only the
// started projection host maintains.
func RunSummaries(ctx context.Context, sys *system.System) ([]RunSummary, error) {
	replay, err := walkRuns(ctx, sys.EventStore(), "")
	if err != nil {
		return nil, err
	}

	return replay.Runs, nil
}

// ResumeSource answers "what did this run already observe?" for --resume:
// the resolved run identity plus the set of domain names it recorded. The
// set holds only domain-name keys, so memory stays proportional to the
// run's domains, not its evidence.
type ResumeSource struct {
	//cqrs-lint:ignore(A032) journal-derived resume identity; branded IDs stop at the command/stream boundary (RunSummary precedent)
	RunID    string
	Observed map[string]struct{}
}

// LoadResumeSource loads the skip-set for a --resume run. runRef is an exact
// run ID or "latest" for the most recently started run. It fails when the
// journal holds no runs at all or the referenced run does not exist.
func LoadResumeSource(ctx context.Context, dsn, runRef string) (ResumeSource, error) {
	sys, err := Boot(ctx, dsn)
	if err != nil {
		return ResumeSource{}, fmt.Errorf("open event store %s: %w", dsn, err)
	}

	defer func() { _ = sys.Close() }()

	if runRef == "" || runRef == "latest" {
		runRef = ""
	}

	runs, err := walkRuns(ctx, sys.EventStore(), runRef)
	if err != nil {
		return ResumeSource{}, err
	}

	if runs.RunID == "" {
		return ResumeSource{}, fmt.Errorf("%w in %s", ErrNoRuns, dsn)
	}

	if !slices.ContainsFunc(runs.Runs, func(r RunSummary) bool { return r.RunID == runs.RunID }) {
		return ResumeSource{}, fmt.Errorf("%w: %s in %s (known runs: %d)", ErrUnknownRun, runRef, dsn, len(runs.Runs))
	}

	source := ResumeSource{RunID: runs.RunID, Observed: map[string]struct{}{}}

	err = walkJournal(ctx, sys.EventStore(), func(evt event.Event) error {
		if evt.Type() != evtDomainObserved {
			return nil
		}

		p, err := decodeObserved(evt)
		if err != nil {
			return fmt.Errorf("decode %s: %w", evtDomainObserved, err)
		}

		if p.RunID == source.RunID {
			source.Observed[p.Domain] = struct{}{}
		}

		return nil
	})
	if err != nil {
		return ResumeSource{}, fmt.Errorf("read event journal: %w", err)
	}

	return source, nil
}

// replayBatchSize bounds journal-walk memory for whole-TLD stores.
const replayBatchSize = 1000

// errUnsupportedStore reports an event store implementing neither the
// seekable-batch nor the plain-journal read interface.
var errUnsupportedStore = errors.New("event store supports neither seeking nor journal reads")

// walkJournal feeds visit every event in the journal in order, batching when
// the store supports seeking so memory stays bounded by the batch. An error
// from visit aborts the walk and is returned unchanged.
func walkJournal(ctx context.Context, store event.Store, visit func(event.Event) error) error {
	if seekable, ok := store.(event.SeekableJournal); ok {
		var after id.EventID

		for {
			batch, err := seekable.ReadFrom(ctx, after, replayBatchSize)
			if err != nil {
				return err
			}

			if len(batch) == 0 {
				return nil
			}

			for _, evt := range batch {
				if err := visit(evt); err != nil {
					return err
				}
			}

			after = batch[len(batch)-1].ID()
		}
	}

	journal, ok := store.(event.Journal)
	if !ok {
		return errUnsupportedStore
	}

	events, err := journal.ReadAll(ctx)
	if err != nil {
		return err
	}

	for _, evt := range events {
		if err := visit(evt); err != nil {
			return err
		}
	}

	return nil
}
