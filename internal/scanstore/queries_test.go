package scanstore_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/larsartmann/go-cqrs-lite/id/v4"
	"github.com/larsartmann/go-cqrs-lite/system/v4"

	"github.com/larsartmann/google-domain-detector/internal/classify"
	"github.com/larsartmann/google-domain-detector/internal/scanstore"
)

// TestQueryReadModelsRoundTrip records two runs on SQLite, reopens the store
// with live projections, and reads both read models through the query layer:
// LatestObservation must return the domain's newest evidence and observation
// count; ListRuns must return both runs newest-first with their tallies.
func TestQueryReadModelsRoundTrip(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	dsn := filepath.Join(t.TempDir(), "scans.db")

	writer, err := scanstore.Open(ctx, dsn)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}

	recorder := scanstore.NewRecorder(writer)

	firstRun := scanstore.NewRunID()
	firstResults := []classify.Result{workspaceResult("acme.com"), failedResult()}
	recordRun(t, ctx, recorder, firstRun, firstResults, "", "", time.Minute)

	if err := writer.GracefulClose(ctx); err != nil {
		t.Fatalf("close writer: %v", err)
	}

	secondRun := scanstore.NewRunID()
	secondResults := []classify.Result{workspaceResult("acme.com"), workspaceResult("beta.io")}

	writer, err = scanstore.Open(ctx, dsn)
	if err != nil {
		t.Fatalf("reopen store: %v", err)
	}

	recorder = scanstore.NewRecorder(writer)
	recordRun(t, ctx, recorder, secondRun, secondResults, firstRun.Get(), "wave-x", time.Second)

	if err := writer.GracefulClose(ctx); err != nil {
		t.Fatalf("close writer: %v", err)
	}

	reader, err := scanstore.Open(ctx, dsn)
	if err != nil {
		t.Fatalf("reopen for queries: %v", err)
	}

	defer func() { _ = reader.Close() }()

	assertLatestObservation(t, ctx, reader, secondRun)
	assertRunViews(t, ctx, reader, firstRun, secondRun, firstResults)
}

// recordRun starts a scan, observes every result, and completes the run.
func recordRun(
	t *testing.T,
	ctx context.Context,
	recorder *scanstore.Recorder,
	runID id.StreamID,
	results []classify.Result,
	resumedFrom, wave string,
	took time.Duration,
) {
	t.Helper()

	if err := recorder.StartScan(ctx, runID, resumedFrom, wave, len(results), 0, 4); err != nil {
		t.Fatalf("start run: %v", err)
	}

	for _, result := range results {
		if err := recorder.Observe(ctx, runID, result); err != nil {
			t.Fatalf("observe: %v", err)
		}
	}

	if err := recorder.CompleteScan(ctx, runID, took, classify.Count(results)); err != nil {
		t.Fatalf("complete run: %v", err)
	}
}

// assertLatestObservation checks that acme.com's projection reflects both
// runs (newest evidence, count 2) and unobserved domains read as absent.
func assertLatestObservation(t *testing.T, ctx context.Context, reader *system.System, lastRun id.StreamID) {
	t.Helper()

	var (
		view     scanstore.ObservationView
		found    bool
		queryErr error
	)

	await(t, func() bool {
		view, found, queryErr = scanstore.LatestObservation(ctx, reader, "acme.com")

		return found && view.Observations >= 2
	})

	if queryErr != nil {
		t.Fatalf("latest observation: %v", queryErr)
	}

	if view.Observations != 2 {
		t.Errorf("acme.com observations = %d, want 2 (one per run)", view.Observations)
	}

	if view.LastRunID != lastRun.Get() {
		t.Errorf("acme.com last run = %s, want %s", view.LastRunID, lastRun.Get())
	}

	if len(view.Evidence) != 1 || view.Evidence[0].Kind != classify.EvidenceMXGoogle {
		t.Errorf("acme.com evidence = %+v, want the MX record", view.Evidence)
	}

	if _, found, err := scanstore.LatestObservation(ctx, reader, "never-scanned.example"); err != nil || found {
		t.Errorf("unobserved domain: found=%v err=%v, want found=false err=nil", found, err)
	}
}

// assertRunViews checks ListRuns returns both runs newest-first with
// provenance and tallies intact.
func assertRunViews(
	t *testing.T,
	ctx context.Context,
	reader *system.System,
	firstRun, secondRun id.StreamID,
	firstResults []classify.Result,
) {
	t.Helper()

	var (
		runs     []scanstore.RunView
		queryErr error
	)

	await(t, func() bool {
		runs, queryErr = scanstore.ListRuns(ctx, reader)

		return queryErr == nil && len(runs) == 2
	})

	if queryErr != nil {
		t.Fatalf("list runs: %v", queryErr)
	}

	if runs[0].RunID != secondRun.Get() || !runs[0].Completed {
		t.Errorf("newest run = %+v, want %s completed", runs[0], secondRun.Get())
	}

	if runs[0].ResumedFrom != firstRun.Get() || runs[0].Wave != "wave-x" {
		t.Errorf("newest run provenance = resumed %q wave %q, want %s / wave-x",
			runs[0].ResumedFrom, runs[0].Wave, firstRun.Get())
	}

	if runs[1].RunID != firstRun.Get() || runs[1].Counts != classify.Count(firstResults) {
		t.Errorf("oldest run = %+v, want %s with %+v", runs[1], firstRun.Get(), classify.Count(firstResults))
	}
}

// await polls cond until it holds or the bounded budget is spent, so tests
// wait for asynchronous projection catch-up without fixed sleeps.
func await(t *testing.T, cond func() bool) {
	t.Helper()

	deadline := time.Now().Add(5 * time.Second)

	for !cond() {
		if time.Now().After(deadline) {
			t.Fatal("condition not met within 5s (projection catch-up stalled?)")
		}

		time.Sleep(10 * time.Millisecond)
	}
}
