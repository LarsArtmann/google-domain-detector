package scanstore_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/larsartmann/go-cqrs-lite/id/v4"

	"github.com/larsartmann/google-domain-detector/internal/classify"
	"github.com/larsartmann/google-domain-detector/internal/scanstore"
)

// scriptedRecorder injects observation failures to exercise the sink retry.
type scriptedRecorder struct {
	RecorderLike struct {
		ObserveFails    int // fail this many Observe calls, then succeed
		AlwaysFail      bool
		ObserveCalls    int
		ObserveRecorded int
	}
}

func (s *scriptedRecorder) Observe(ctx context.Context, runID id.StreamID, result classify.Result) error {
	s.RecorderLike.ObserveCalls++

	if s.RecorderLike.AlwaysFail || s.RecorderLike.ObserveFails > 0 {
		s.RecorderLike.ObserveFails--

		return errors.New("sqlite is locked")
	}

	s.RecorderLike.ObserveRecorded++

	return nil
}

func (s *scriptedRecorder) CompleteScan(context.Context, id.StreamID, time.Duration, classify.Counts) error {
	return nil
}

// TestSinkRetriesTransientStoreErrors pins 11.3a: a couple of locked-store
// failures per observation are survived; everything lands in order.
func TestSinkRetriesTransientStoreErrors(t *testing.T) {
	t.Parallel()

	recorder := &scriptedRecorder{}
	recorder.RecorderLike.ObserveFails = 2 // fail twice, then succeed

	sink := scanstore.NewSink(recorder, scanstore.NewRunID(), 4)

	for _, name := range []string{"a.example", "b.example", "c.example"} {
		sink.OnResult(classify.Result{Domain: name})
	}

	if err := sink.Close(context.Background(), time.Second, classify.Counts{Scanned: 3}); err != nil {
		t.Fatalf("transient failures must not surface: %v", err)
	}

	if recorder.RecorderLike.ObserveRecorded != 3 {
		t.Errorf("recorded %d of 3 observations", recorder.RecorderLike.ObserveRecorded)
	}
}

// TestSinkFailsLoudlyWithDropCount pins 11.3b: a permanently broken store
// yields a Close error naming the exact number of lost observations — a
// 2.5M-domain journal can never lose rows silently.
func TestSinkFailsLoudlyWithDropCount(t *testing.T) {
	t.Parallel()

	recorder := &scriptedRecorder{}
	recorder.RecorderLike.AlwaysFail = true

	sink := scanstore.NewSink(recorder, scanstore.NewRunID(), 4)

	for _, name := range []string{"a.example", "b.example", "c.example"} {
		sink.OnResult(classify.Result{Domain: name})
	}

	err := sink.Close(context.Background(), time.Second, classify.Counts{Scanned: 3})
	if err == nil {
		t.Fatal("permanent store failure must fail Close")
	}

	if !strings.Contains(err.Error(), "3 observations dropped") {
		t.Errorf("Close error must name the dropped count, got: %v", err)
	}
}
