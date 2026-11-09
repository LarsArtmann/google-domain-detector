// Package scanstore makes scan runs event-sourced via go-cqrs-lite: every
// observation is appended as a domain event (one stream per domain, one per
// run) and the whole report can be rebuilt from the journal alone. All
// durable I/O goes through the system composition root (ADR-0123): the
// caller only picks the SQLite file.
package scanstore

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/larsartmann/go-cqrs-lite/command/v4"
	"github.com/larsartmann/go-cqrs-lite/decider/v4"
	"github.com/larsartmann/go-cqrs-lite/event/v4"
	"github.com/larsartmann/go-cqrs-lite/id/v4"
	"github.com/larsartmann/go-cqrs-lite/metaengine/projectionadapter/v4"
	_ "github.com/larsartmann/go-cqrs-lite/metaengine/sqliteengine/v4" // register the "sqlite" driver
	"github.com/larsartmann/go-cqrs-lite/metaengine/v4"
	"github.com/larsartmann/go-cqrs-lite/record/v4"
	"github.com/larsartmann/go-cqrs-lite/system/v4"

	"github.com/larsartmann/google-domain-detector/internal/classify"
)

// Stream types: one stream per scanned domain, one per scan run.
const (
	StreamTypeDomain  = "Domain"
	StreamTypeScanRun = "ScanRun"
)

// Event types persisted to the journal.
const (
	evtScanStarted     = event.Type("scan.started")
	evtDomainObserved  = event.Type("scan.domain_observed")
	evtScanCompleted   = event.Type("scan.completed")
	cmdStartScan       = command.Type("scan.start")
	cmdRecordSingleObs = command.Type("scan.observe")
	cmdCompleteScan    = command.Type("scan.complete")
)

// Read-model collections (metaengine Map queries).
const (
	CollectionObservations = "observation_views"
	CollectionRuns         = "run_views"
)

// enginePrimary is the single engine name used by the deployment config.
const enginePrimary = "primary"

// ScanStartedPayload opens a run.
type ScanStartedPayload struct {
	RunID       string    `json:"run_id"`
	StartedAt   time.Time `json:"started_at"`
	InputDomain int       `json:"input_domains"`
	Skipped     int       `json:"skipped"`
	Workers     int       `json:"workers"`
}

// DomainObservedPayload records everything one scan learned about one
// domain. Verdicts are deliberately absent: they are derived from evidence
// (classify.Result methods), and storing derived state next to the facts it
// comes from would be a split brain.
type DomainObservedPayload struct {
	Domain     string              `json:"domain"`
	RunID      string              `json:"run_id"`
	Evidence   []classify.Evidence `json:"evidence"`
	Err        string              `json:"error,omitempty"`
	ObservedAt time.Time           `json:"observed_at"`
}

// ScanCompletedPayload closes a run with the scanner's own tally.
type ScanCompletedPayload struct {
	RunID       string         `json:"run_id"`
	CompletedAt time.Time      `json:"completed_at"`
	Duration    time.Duration  `json:"duration"`
	Counts      classify.Counts `json:"counts"`
}

// StartScanCmd opens a run on its own stream.
type StartScanCmd struct {
	*command.BasicCommand

	InputDomain int
	Skipped     int
	Workers     int
}

// RecordObservationCmd appends one observation to the domain's stream.
type RecordObservationCmd struct {
	*command.BasicCommand

	RunID  string
	Result classify.Result
}

// CompleteScanCmd closes the run stream.
type CompleteScanCmd struct {
	*command.BasicCommand

	Duration time.Duration
	Counts   classify.Counts
}

// ScanRunState is the write-side state of one run stream.
type ScanRunState struct {
	Started   bool
	Completed bool
	Counts    classify.Counts
}

// DomainState is the write-side state of one domain stream.
type DomainState struct {
	Observations int
}

func applyDomain(s DomainState, evt event.Event) (DomainState, error) {
	if evt.Type() != evtDomainObserved {
		return s, nil
	}

	s.Observations++

	return s, nil
}

func applyScanRun(s ScanRunState, evt event.Event) (ScanRunState, error) {
	switch evt.Type() {
	case evtScanStarted:
		s.Started = true
	case evtScanCompleted:
		p, err := event.DecodePayloadAuto[ScanCompletedPayload](evt)
		if err != nil {
			return s, err
		}

		s.Completed = true
		s.Counts = p.Counts
	}

	return s, nil
}

// ObservationView is the read model for one domain: its latest observation.
type ObservationView struct {
	Domain       string              `json:"domain"`
	LastRunID    string              `json:"last_run_id"`
	Evidence     []classify.Evidence `json:"evidence"`
	Err          string              `json:"error,omitempty"`
	ObservedAt   time.Time           `json:"observed_at"`
	Observations int                  `json:"observations"`
}

// RunView is the read model for one run stream.
type RunView struct {
	RunID     string          `json:"run_id"`
	Started   bool            `json:"started"`
	Completed bool            `json:"completed"`
	Counts    classify.Counts `json:"counts"`
}

// Domain declares the write model, events, and read models. The production
// binary and the systemscenario tests boot the SAME declaration, so the
// tests cannot drift from production wiring.
func Domain() system.DomainConfig {
	observations, runs, decoder := projections()

	return system.DomainConfig{
		Events: []event.Type{evtScanStarted, evtDomainObserved, evtScanCompleted},
		Commands: func(sys *system.System) {
			mustRegister(system.RegisterDecider(sys, StreamTypeDomain, decider.Decider[DomainState]{
				Initial: DomainState{},
				Apply:   applyDomain,
			}))

			mustRegister(system.RegisterDecider(sys, StreamTypeScanRun, decider.Decider[ScanRunState]{
				Initial: ScanRunState{},
				Apply:   applyScanRun,
			}))

			mustRegister(system.RegisterCommand[RecordObservationCmd, DomainState](sys, cmdRecordSingleObs,
				func(cmdCtx context.Context, cmd RecordObservationCmd) system.Op[DomainState] {
					return system.Execute(cmdCtx, cmd.StreamID(), StreamTypeDomain,
						func(_ DomainState, v event.Version) ([]event.Event, error) {
							evt, err := event.New(evtDomainObserved, cmd.StreamID(), StreamTypeDomain,
								v.Increment(), DomainObservedPayload{
									Domain:     cmd.Result.Domain,
									RunID:      cmd.RunID,
									Evidence:   cmd.Result.Evidences,
									Err:        cmd.Result.Err,
									ObservedAt: time.Now(),
								})
							if err != nil {
								return nil, err
							}

							return []event.Event{evt}, nil
						})
				}))

			mustRegister(system.RegisterCommand[StartScanCmd, ScanRunState](sys, cmdStartScan,
				func(cmdCtx context.Context, cmd StartScanCmd) system.Op[ScanRunState] {
					return system.Execute(cmdCtx, cmd.StreamID(), StreamTypeScanRun,
						func(s ScanRunState, v event.Version) ([]event.Event, error) {
							if s.Started { // idempotent: a replayed start emits nothing
								return nil, nil
							}

							evt, err := event.New(evtScanStarted, cmd.StreamID(), StreamTypeScanRun,
								v.Increment(), ScanStartedPayload{
									RunID:       cmd.StreamID().Get(),
									StartedAt:   time.Now(),
									InputDomain: cmd.InputDomain,
									Skipped:     cmd.Skipped,
									Workers:     cmd.Workers,
								})
							if err != nil {
								return nil, err
							}

							return []event.Event{evt}, nil
						})
				}))

			mustRegister(system.RegisterCommand[CompleteScanCmd, ScanRunState](sys, cmdCompleteScan,
				func(cmdCtx context.Context, cmd CompleteScanCmd) system.Op[ScanRunState] {
					return system.Execute(cmdCtx, cmd.StreamID(), StreamTypeScanRun,
						func(s ScanRunState, v event.Version) ([]event.Event, error) {
							if s.Completed { // idempotent: a replayed completion emits nothing
								return nil, nil
							}

							evt, err := event.New(evtScanCompleted, cmd.StreamID(), StreamTypeScanRun,
								v.Increment(), ScanCompletedPayload{
									RunID:       cmd.StreamID().Get(),
									CompletedAt: time.Now(),
									Duration:    cmd.Duration,
									Counts:      cmd.Counts,
								})
							if err != nil {
								return nil, err
							}

							return []event.Event{evt}, nil
						})
				}))
		},
		Projections:           []system.ProjectionDeclaration{system.RawQuery(observations), system.RawQuery(runs)},
		ProjectionTypeDecoder: decoder,
	}
}

// Deployment pins the engine topology: one SQLite file for journal and
// projections. Swapping engines later is this function alone.
func Deployment(dsn string) system.DeploymentConfig {
	return system.DeploymentConfig{
		Engines: map[string]system.EngineConfig{
			enginePrimary: {Driver: "sqlite", DSN: dsn},
		},
		Instances: []system.InstanceConfig{
			{Role: system.RoleSourceOfTruth, Engine: enginePrimary},
			{Role: system.RoleProjections, Engine: enginePrimary},
		},
	}
}

// Open boots the composition root on dsn and starts the projection host.
func Open(ctx context.Context, dsn string) (*system.System, error) {
	sys, err := system.New(ctx, Domain(), Deployment(dsn))
	if err != nil {
		return nil, fmt.Errorf("boot event-sourced scan store: %w", err)
	}

	if err := sys.Start(ctx); err != nil {
		_ = sys.Close()

		return nil, fmt.Errorf("start event-sourced scan store: %w", err)
	}

	return sys, nil
}

// Recorder dispatches scan commands; it is the only write API callers need.
// Dispatches are sequential per Recorder, which keeps journal order stable
// for replay.
type Recorder struct {
	sys *system.System
}

// NewRecorder wraps a booted system.
func NewRecorder(sys *system.System) *Recorder {
	return &Recorder{sys: sys}
}

// NewRunID mints the identity of one scan run.
func NewRunID() id.StreamID {
	return id.NewStreamID()
}

// StartScan opens the run.
func (rec *Recorder) StartScan(ctx context.Context, runID id.StreamID, inputDomain, skipped, workers int) error {
	basic, err := command.New(cmdStartScan, runID)
	if err != nil {
		return fmt.Errorf("build %s command: %w", cmdStartScan, err)
	}

	cmd := StartScanCmd{BasicCommand: basic, InputDomain: inputDomain, Skipped: skipped, Workers: workers}

	return rec.sys.CommandDispatcher().Dispatch(ctx, cmd)
}

// Observe appends one domain observation.
func (rec *Recorder) Observe(ctx context.Context, runID id.StreamID, result classify.Result) error {
	streamID, err := StreamForDomain(result.Domain)
	if err != nil {
		return err
	}

	basic, err := command.New(cmdRecordSingleObs, streamID)
	if err != nil {
		return fmt.Errorf("build %s command: %w", cmdRecordSingleObs, err)
	}

	cmd := RecordObservationCmd{BasicCommand: basic, RunID: runID.Get(), Result: result}

	return rec.sys.CommandDispatcher().Dispatch(ctx, cmd)
}

// CompleteScan closes the run with its tally.
func (rec *Recorder) CompleteScan(
	ctx context.Context,
	runID id.StreamID,
	duration time.Duration,
	counts classify.Counts,
) error {
	basic, err := command.New(cmdCompleteScan, runID)
	if err != nil {
		return fmt.Errorf("build %s command: %w", cmdCompleteScan, err)
	}

	cmd := CompleteScanCmd{BasicCommand: basic, Duration: duration, Counts: counts}

	return rec.sys.CommandDispatcher().Dispatch(ctx, cmd)
}

// StreamForDomain returns the stable stream identity of a domain, so
// re-scans append to the same stream and keep their history.
func StreamForDomain(domainName string) (id.StreamID, error) {
	streamID, err := id.ParseStreamID(domainName)
	if err != nil {
		return streamID, fmt.Errorf("stream id for domain %q: %w", domainName, err)
	}

	return streamID, nil
}

// RunSummary is one run as seen while walking the journal.
type RunSummary struct {
	RunID     string
	StartedAt time.Time
	Completed bool
	Counts    classify.Counts
}

// Replay is the rebuild-from-events result: the per-domain observations of
// one run (last observation wins) plus every run found in the journal.
type Replay struct {
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

// replayBatchSize bounds journal-walk memory for whole-TLD stores.
const replayBatchSize = 1000

func projections() (
	metaengine.QueryDecl[struct{}, ObservationView],
	metaengine.QueryDecl[struct{}, RunView],
	*projectionadapter.TypeDecoder,
) {
	observations := metaengine.Query[struct{}, ObservationView](CollectionObservations,
		metaengine.OnRecordTyped(
			string(evtDomainObserved),
			projectionadapter.EventWithID[DomainObservedPayload]{},
			func(_ record.Record, e projectionadapter.EventWithID[DomainObservedPayload]) (string, ObservationView) {
				return e.ID, observationViewFrom(e.Payload, 1)
			},
		),
		metaengine.OnRecordTyped(
			string(evtDomainObserved),
			projectionadapter.EventWithID[DomainObservedPayload]{},
			func(_ record.Record, e projectionadapter.EventWithID[DomainObservedPayload], prev ObservationView) ObservationView {
				return observationViewFrom(e.Payload, prev.Observations+1)
			},
		),
	)

	runs := metaengine.Query[struct{}, RunView](CollectionRuns,
		metaengine.OnRecordTyped(
			string(evtScanStarted),
			projectionadapter.EventWithID[ScanStartedPayload]{},
			func(_ record.Record, e projectionadapter.EventWithID[ScanStartedPayload]) (string, RunView) {
				return e.ID, RunView{RunID: e.ID, Started: true}
			},
		),
		metaengine.OnRecordTyped(
			string(evtScanCompleted),
			projectionadapter.EventWithID[ScanCompletedPayload]{},
			func(_ record.Record, e projectionadapter.EventWithID[ScanCompletedPayload], prev RunView) RunView {
				prev.Completed = true
				prev.Counts = e.Payload.Counts

				return prev
			},
		),
	)

	decoder := projectionadapter.NewTypeDecoder(
		projectionadapter.Register(evtScanStarted, ScanStartedPayload{}),
		projectionadapter.Register(evtDomainObserved, DomainObservedPayload{}),
		projectionadapter.Register(evtScanCompleted, ScanCompletedPayload{}),
	)

	return observations, runs, decoder
}

func observationViewFrom(p DomainObservedPayload, observations int) ObservationView {
	return ObservationView{
		Domain:       p.Domain,
		LastRunID:    p.RunID,
		Evidence:     p.Evidence,
		Err:          p.Err,
		ObservedAt:   p.ObservedAt,
		Observations: observations,
	}
}

func mustRegister(err error) {
	if err != nil {
		panic(fmt.Sprintf("scanstore composition: %v", err))
	}
}
