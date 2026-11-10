package scanstore_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

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
	firstResults := []classify.Result{workspaceResult("acme.com"), failedResult("gone.example")}

	if err := recorder.StartScan(ctx, firstRun, "", "", len(firstResults), 0, 4); err != nil {
		t.Fatalf("start first run: %v", err)
	}

	for _, result := range firstResults {
		if err := recorder.Observe(ctx, firstRun, result); err != nil {
			t.Fatalf("observe: %v", err)
		}
	}

	if err := recorder.CompleteScan(ctx, firstRun, time.Minute, classify.Count(firstResults)); err != nil {
		t.Fatalf("complete first run: %v", err)
	}

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

	if err := recorder.StartScan(ctx, secondRun, firstRun.Get(), "wave-x", len(secondResults), 0, 4); err != nil {
		t.Fatalf("start second run: %v", err)
	}

	for _, result := range secondResults {
		if err := recorder.Observe(ctx, secondRun, result); err != nil {
			t.Fatalf("observe: %v", err)
		}
	}

	if err := recorder.CompleteScan(ctx, secondRun, time.Second, classify.Count(secondResults)); err != nil {
		t.Fatalf("complete second run: %v", err)
	}

	if err := writer.GracefulClose(ctx); err != nil {
		t.Fatalf("close writer: %v", err)
	}

	reader, err := scanstore.Open(ctx, dsn)
	if err != nil {
		t.Fatalf("reopen for queries: %v", err)
	}

	defer func() { _ = reader.Close() }()

	var (
		view     scanstore.ObservationView
		found    bool
		queryErr error
	)

	await(t, func() bool {
		view, found, queryErr = scanstore.LatestObservation(ctx, reader, "acme.com")

		return found
	})

	if queryErr != nil {
		t.Fatalf("latest observation: %v", queryErr)
	}

	if view.Observations != 2 {
		t.Errorf("acme.com observations = %d, want 2 (one per run)", view.Observations)
	}

	if view.LastRunID != secondRun.Get() {
		t.Errorf("acme.com last run = %s, want %s", view.LastRunID, secondRun.Get())
	}

	if len(view.Evidence) != 1 || view.Evidence[0].Kind != classify.EvidenceMXGoogle {
		t.Errorf("acme.com evidence = %+v, want the MX record", view.Evidence)
	}

	if _, found, err := scanstore.LatestObservation(ctx, reader, "never-scanned.example"); err != nil || found {
		t.Errorf("unobserved domain: found=%v err=%v, want found=false err=nil", found, err)
	}

	var runs []scanstore.RunView

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
