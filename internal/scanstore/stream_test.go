package scanstore_test

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/larsartmann/google-domain-detector/internal/classify"
	"github.com/larsartmann/google-domain-detector/internal/scanstore"
)

// TestStreamObservationsTenThousandEvents is the T8.9 gate: a 10k-observation
// journal streams every observation exactly once — the memory shape replay
// needs for whole-TLD stores.
func TestStreamObservationsTenThousandEvents(t *testing.T) {
	t.Parallel()

	if testing.Short() {
		t.Skip("10k-event journal seed is slow")
	}

	ctx := context.Background()
	dsn := filepath.Join(t.TempDir(), "scans.db")

	writer, err := scanstore.Open(ctx, dsn)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}

	recorder := scanstore.NewRecorder(writer)

	runID := scanstore.NewRunID()

	const total = 10000

	if err := recorder.StartScan(ctx, runID, "", "", total, 0, 8); err != nil {
		t.Fatalf("start run: %v", err)
	}

	for i := range total {
		if err := recorder.Observe(ctx, runID, classify.Result{Domain: fmt.Sprintf("d%d.example", i)}); err != nil {
			t.Fatalf("observe %d: %v", i, err)
		}
	}

	if err := recorder.CompleteScan(ctx, runID, time.Minute, classify.Counts{Scanned: total}); err != nil {
		t.Fatalf("complete run: %v", err)
	}

	if err := writer.GracefulClose(ctx); err != nil {
		t.Fatalf("close writer: %v", err)
	}

	reader, err := scanstore.Boot(ctx, dsn)
	if err != nil {
		t.Fatalf("boot reader: %v", err)
	}

	defer func() { _ = reader.Close() }()

	seen := map[string]int{}

	summary, err := scanstore.StreamObservations(ctx, reader, "", func(r classify.Result) error {
		seen[r.Domain]++

		return nil
	})
	if err != nil {
		t.Fatalf("stream observations: %v", err)
	}

	if len(seen) != total {
		t.Errorf("streamed %d distinct domains, want %d", len(seen), total)
	}

	for name, count := range seen {
		if count != 1 {
			t.Fatalf("domain %s streamed %d times", name, count)
		}
	}

	if summary.Counts.Scanned != total {
		t.Errorf("summary scanned = %d, want %d", summary.Counts.Scanned, total)
	}
}

// TestStreamObservationsMatchesReplay pins the equivalence of the streaming
// and buffered journal readers on the same store: same run selection, same
// results, same order.
func TestStreamObservationsMatchesReplay(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	dsn := filepath.Join(t.TempDir(), "scans.db")

	writer, err := scanstore.Open(ctx, dsn)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}

	recorder := scanstore.NewRecorder(writer)

	firstRun := scanstore.NewRunID()
	seedResults := []classify.Result{
		workspaceResult("acme.com"),
		failedResult(),
		workspaceResult("beta.io"),
	}

	if err := recorder.StartScan(ctx, firstRun, "", "", len(seedResults), 0, 4); err != nil {
		t.Fatalf("start run: %v", err)
	}

	for _, result := range seedResults {
		if err := recorder.Observe(ctx, firstRun, result); err != nil {
			t.Fatalf("observe %s: %v", result.Domain, err)
		}
	}

	if err := recorder.CompleteScan(ctx, firstRun, time.Second, classify.Count(seedResults)); err != nil {
		t.Fatalf("complete run: %v", err)
	}

	if err := writer.GracefulClose(ctx); err != nil {
		t.Fatalf("close writer: %v", err)
	}

	reader, err := scanstore.Boot(ctx, dsn)
	if err != nil {
		t.Fatalf("boot reader: %v", err)
	}

	defer func() { _ = reader.Close() }()

	buffered, err := scanstore.ReplayJournal(ctx, reader, "")
	if err != nil {
		t.Fatalf("buffered replay: %v", err)
	}

	var streamed []classify.Result

	summary, err := scanstore.StreamObservations(ctx, reader, "", func(r classify.Result) error {
		streamed = append(streamed, r)

		return nil
	})
	if err != nil {
		t.Fatalf("stream observations: %v", err)
	}

	assertResults(t, streamed, buffered.Results)

	if summary.RunID != buffered.RunID || summary.Counts.Scanned != len(seedResults) || !summary.Completed {
		t.Errorf("streamed summary %+v does not match the buffered run %s", summary, buffered.RunID)
	}
}

// TestStreamObservationsEmptyStore covers the no-runs sentinel: streaming
// from an empty journal must fail with ErrNoRuns, not an empty success.
func TestStreamObservationsEmptyStore(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	dsn := filepath.Join(t.TempDir(), "scans.db")

	if _, err := scanstore.Open(ctx, dsn); err != nil {
		t.Fatalf("open store: %v", err)
	}

	reader, err := scanstore.Boot(ctx, dsn)
	if err != nil {
		t.Fatalf("boot reader: %v", err)
	}

	defer func() { _ = reader.Close() }()

	if _, err := scanstore.StreamObservations(
		ctx,
		reader,
		"",
		func(classify.Result) error { return nil },
	); !errors.Is(
		err,
		scanstore.ErrNoRuns,
	) {
		t.Errorf("empty store error = %v, want ErrNoRuns", err)
	}
}

// TestStreamObservationsTenThousandEvents is the T8.9 gate: a 10k-observation
// journal streams every observation exactly once — the memory shape replay
