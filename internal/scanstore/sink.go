package scanstore

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/larsartmann/go-cqrs-lite/id/v4"

	"github.com/larsartmann/google-domain-detector/internal/classify"
)

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
