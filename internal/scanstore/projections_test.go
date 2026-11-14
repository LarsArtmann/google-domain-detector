package scanstore

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/larsartmann/google-domain-detector/internal/classify"
)

// TestWaitProjectionsSettledMakesUnfoldedJournalVisible pins the staleness
// race: a journal written WITHOUT a running projection host (Boot-shaped
// writer, or any unclean close that left the tail unfoldled) must become
// fully visible to read-model queries only AFTER WaitProjectionsSettled.
// Before the wait existed, Open + immediate ListRuns deterministically saw
// zero runs on iteration 0.
func TestWaitProjectionsSettledMakesUnfoldedJournalVisible(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := filepath.Join(t.TempDir(), "s.db")

	writer, err := Boot(ctx, store)
	if err != nil {
		t.Fatalf("boot writer: %v", err)
	}

	rec := NewRecorder(writer)
	runID := NewRunID()

	if err := rec.StartScan(ctx, runID, "", "wave-stale", 1, 0, 1); err != nil {
		t.Fatalf("start: %v", err)
	}

	if err := rec.Observe(ctx, runID, classify.Result{Domain: "a.example"}); err != nil {
		t.Fatalf("observe: %v", err)
	}

	if err := writer.GracefulClose(ctx); err != nil {
		t.Fatalf("close writer: %v", err)
	}

	reader, err := Open(ctx, store)
	if err != nil {
		t.Fatalf("open reader: %v", err)
	}

	defer func() { _ = reader.Close() }()

	// Pin the hazard first: with NO wait the read model may be stale.
	// (Not asserted — the race is timing-dependent; the wait is the fix.)

	if err := WaitProjectionsSettled(ctx, reader); err != nil {
		t.Fatalf("wait for settled projections: %v", err)
	}

	runs, err := ListRuns(ctx, reader)
	if err != nil {
		t.Fatalf("list runs: %v", err)
	}

	if len(runs) != 1 {
		t.Fatalf("settled read model must show the run, saw %d", len(runs))
	}

	if runs[0].RunID != runID.Get() || runs[0].Wave != "wave-stale" {
		t.Errorf("run identity lost across the unfoldled-journal round trip: %+v", runs[0])
	}
}

// TestWaitProjectionsSettledHonorsContext pins cancellation: a canceled
// context aborts the wait with an error instead of blocking the budget.
func TestWaitProjectionsSettledHonorsContext(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := filepath.Join(t.TempDir(), "s.db")

	writer, err := Boot(ctx, store)
	if err != nil {
		t.Fatalf("boot writer: %v", err)
	}

	rec := NewRecorder(writer)
	runID := NewRunID()

	if err := rec.StartScan(ctx, runID, "", "", 1, 0, 1); err != nil {
		t.Fatalf("start: %v", err)
	}

	if err := writer.GracefulClose(ctx); err != nil {
		t.Fatalf("close writer: %v", err)
	}

	reader, err := Open(ctx, store)
	if err != nil {
		t.Fatalf("open reader: %v", err)
	}

	defer func() { _ = reader.Close() }()

	cancelCtx, cancel := context.WithCancel(ctx)
	cancel()

	err = WaitProjectionsSettled(cancelCtx, reader)
	if err == nil {
		t.Fatal("canceled wait must fail")
	}
}
