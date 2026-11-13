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
	_ "github.com/larsartmann/go-cqrs-lite/metaengine/sqliteengine/v4" // register the "sqlite" driver
	"github.com/larsartmann/go-cqrs-lite/system/v4"

	"github.com/larsartmann/google-domain-detector/internal/classify"
)

// enginePrimary is the single engine name used by the deployment config.
const enginePrimary = "primary"

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

			mustRegister(system.RegisterCommand[RecordObservationCmd, DomainState](sys, CmdObserve,
				func(cmdCtx context.Context, cmd RecordObservationCmd) system.Op[DomainState] {
					return system.Execute(cmdCtx, cmd.StreamID(), StreamTypeDomain,
						func(_ DomainState, v event.Version) ([]event.Event, error) {
							payload := DomainObservedPayload{
								Domain:     cmd.Result.Domain,
								RunID:      cmd.RunID,
								Evidence:   cmd.Result.Evidences,
								Err:        cmd.Result.Err,
								ErrKind:    cmd.Result.ErrKind,
								Ruleset:    classify.CurrentRuleset,
								ObservedAt: event.NewInstantNow(),
							}
							sanitizeObservedPayload(&payload)

							evt, err := event.New(evtDomainObserved, cmd.StreamID(), StreamTypeDomain,
								v.Increment(), payload, event.WithSchemaVersion(1))
							if err != nil {
								return nil, err
							}

							return []event.Event{evt}, nil
						})
				}))

			mustRegister(system.RegisterCommand[StartScanCmd, ScanRunState](sys, CmdStartScan,
				func(cmdCtx context.Context, cmd StartScanCmd) system.Op[ScanRunState] {
					return system.Execute(cmdCtx, cmd.StreamID(), StreamTypeScanRun,
						func(s ScanRunState, v event.Version) ([]event.Event, error) {
							if s.Started { // idempotent: a replayed start emits nothing
								return nil, nil
							}

							evt, err := event.New(evtScanStarted, cmd.StreamID(), StreamTypeScanRun,
								v.Increment(), ScanStartedPayload{
									RunID:       cmd.StreamID().Get(),
									StartedAt:   event.NewInstantNow(),
									InputDomain: cmd.InputDomain,
									Skipped:     cmd.Skipped,
									Workers:     cmd.Workers,
									ResumedFrom: cmd.ResumedFrom,
									Wave:        cmd.Wave,
									Ruleset:     classify.CurrentRuleset,
								}, event.WithSchemaVersion(1))
							if err != nil {
								return nil, err
							}

							return []event.Event{evt}, nil
						})
				}))

			mustRegister(system.RegisterCommand[CompleteScanCmd, ScanRunState](sys, CmdCompleteScan,
				func(cmdCtx context.Context, cmd CompleteScanCmd) system.Op[ScanRunState] {
					return system.Execute(cmdCtx, cmd.StreamID(), StreamTypeScanRun,
						func(s ScanRunState, v event.Version) ([]event.Event, error) {
							if s.Completed { // idempotent: a replayed completion emits nothing
								return nil, nil
							}

							evt, err := event.New(evtScanCompleted, cmd.StreamID(), StreamTypeScanRun,
								v.Increment(), ScanCompletedPayload{
									RunID:       cmd.StreamID().Get(),
									CompletedAt: event.NewInstantNow(),
									Duration:    cmd.Duration,
									Counts:      cmd.Counts,
								}, event.WithSchemaVersion(1))
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
	sys, err := Boot(ctx, dsn)
	if err != nil {
		return nil, err
	}

	if err := sys.Start(ctx); err != nil {
		if closeErr := sys.Close(); closeErr != nil {
			return nil, fmt.Errorf("start event-sourced scan store: %w (close: %w)", err, closeErr)
		}

		return nil, fmt.Errorf("start event-sourced scan store: %w", err)
	}

	return sys, nil
}

// Boot composes the system without starting the projection host. Reads that
// walk the journal (replay) do not need read models and should not pay for
// folding the whole store first.
func Boot(ctx context.Context, dsn string) (*system.System, error) {
	sys, err := system.New(ctx, Domain(), Deployment(dsn))
	if err != nil {
		return nil, fmt.Errorf("boot event-sourced scan store: %w", err)
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

// StartScan opens the run. resumedFrom names the run this run resumes
// (empty for a fresh scan); wave names the census wave manifest the run
// belongs to (empty for ad-hoc runs).
func (rec *Recorder) StartScan(
	ctx context.Context,
	runID id.StreamID,
	resumedFrom string,
	wave string,
	inputDomain, skipped, workers int,
) error {
	basic, err := command.New(CmdStartScan, runID)
	if err != nil {
		return fmt.Errorf("build %s command: %w", CmdStartScan, err)
	}

	cmd := StartScanCmd{
		BasicCommand: basic,
		InputDomain:  inputDomain,
		Skipped:      skipped,
		Workers:      workers,
		ResumedFrom:  resumedFrom,
		Wave:         wave,
	}

	return rec.sys.CommandDispatcher().Dispatch(ctx, cmd)
}

// Observe appends one domain observation.
func (rec *Recorder) Observe(ctx context.Context, runID id.StreamID, result classify.Result) error {
	streamID, err := StreamForDomain(result.Domain)
	if err != nil {
		return err
	}

	basic, err := command.New(CmdObserve, streamID)
	if err != nil {
		return fmt.Errorf("build %s command: %w", CmdObserve, err)
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
	basic, err := command.New(CmdCompleteScan, runID)
	if err != nil {
		return fmt.Errorf("build %s command: %w", CmdCompleteScan, err)
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

// runRecorder is the slice of Recorder the Sink needs; tests inject
// failures through it.
type runRecorder interface {
	Observe(ctx context.Context, runID id.StreamID, result classify.Result) error
	CompleteScan(ctx context.Context, runID id.StreamID, duration time.Duration, counts classify.Counts) error
}

// Sink bridges the scan's worker pool to the recorder: results land on a
// buffered channel and are recorded sequentially on one goroutine, so the
// journal order stays stable and writers never contend. Each observation
// gets a bounded retry with backoff (a transient SQLite hiccup must not cost
// data); once the budget is spent the sink stops writing but keeps draining
// so a broken store never blocks the DNS pool. Close then reports the first
// error together with the exact count of dropped observations — never a
// silent gap in the journal.
type Sink struct {
	recorder runRecorder
	runID    id.StreamID
	events   chan classify.Result
	done     chan struct{}
	firstErr error
	dropped  int
}

// NewSink starts recording observations for runID. depth bounds the buffer
// between the DNS pool and the store. Observations deliberately record on a
// detached context: a canceled scan must still journal every completed
// observation and close its run — the journal is the --resume checkpoint.
func NewSink(recorder runRecorder, runID id.StreamID, depth int) *Sink {
	sink := &Sink{
		recorder: recorder,
		runID:    runID,
		events:   make(chan classify.Result, depth),
		done:     make(chan struct{}),
	}

	go sink.record()

	return sink
}

// OnResult is the scan.Options.OnResult hook.
func (sink *Sink) OnResult(result classify.Result) {
	sink.events <- result
}

// Close drains every buffered result, completes the run, and returns the
// first recording error plus the dropped-observation count, if any.
func (sink *Sink) Close(ctx context.Context, duration time.Duration, counts classify.Counts) error {
	close(sink.events)
	<-sink.done

	// Completing the journal entry is a local write: a canceled scan run
	// (Ctrl-C) still closes its run record.
	completeErr := sink.recorder.CompleteScan(context.WithoutCancel(ctx), sink.runID, duration, counts)

	err := errors.Join(sink.firstErr, completeErr)
	if err == nil {
		return nil
	}

	if sink.dropped > 0 {
		return fmt.Errorf("event store recording failed, %d observations dropped: %w", sink.dropped, err)
	}

	return err
}

// observeAttempts and observeBackoff bound the per-observation retry: a
// transient store error costs at most two short pauses, a permanent one
// fails after three attempts.
const (
	observeAttempts = 3
	observeBackoff  = 25 * time.Millisecond
)

func (sink *Sink) record() {
	defer close(sink.done)

	ctx := context.Background()

	for result := range sink.events {
		if sink.firstErr != nil { // store is broken: keep draining, count the loss
			sink.dropped++

			continue
		}

		if err := sink.observe(ctx, result); err != nil {
			sink.firstErr = fmt.Errorf("record observation for %s: %w", result.Domain, err)
			sink.dropped = 1
		}
	}
}

func (sink *Sink) observe(ctx context.Context, result classify.Result) error {
	backoff := observeBackoff

	var err error

	for attempt := range observeAttempts {
		if attempt > 0 {
			time.Sleep(backoff)
			backoff *= 2
		}

		if err = sink.recorder.Observe(ctx, sink.runID, result); err == nil {
			return nil
		}
	}

	return err
}

func mustRegister(err error) {
	if err != nil {
		panic(fmt.Sprintf("scanstore composition: %v", err))
	}
}
