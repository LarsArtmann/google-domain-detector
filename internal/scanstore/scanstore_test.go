package scanstore_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/larsartmann/go-cqrs-lite/command/v4"
	"github.com/larsartmann/go-cqrs-lite/metaengine/v4"
	"github.com/larsartmann/go-cqrs-lite/systemscenario/v4"

	"github.com/larsartmann/google-domain-detector/internal/classify"
	"github.com/larsartmann/google-domain-detector/internal/scanstore"
)

func workspaceResult(domainName string) classify.Result {
	return classify.Result{
		Domain: domainName,
		Evidences: []classify.Evidence{
			{
				Kind:     classify.EvidenceMXGoogle,
				Platform: classify.PlatformWorkspace,
				Weight:   classify.WeightStrong,
				Host:     domainName,
				Record:   "smtp.google.com.",
			},
		},
	}
}

func failedResult(domainName string) classify.Result {
	return classify.Result{Domain: domainName, Err: "domain not found: nxdomain", ErrKind: classify.ErrorNXDomain}
}

// TestCommandsProjectReadModels runs the production DomainConfig through the
// systemscenario harness: dispatch the real commands, assert the journal
// diff, and await the observation read model.
func TestCommandsProjectReadModels(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	sc := systemscenario.System(t, ctx, scanstore.Domain(), systemscenario.Memory())

	runID := scanstore.NewRunID()

	startBasic, err := command.New(scanstore.CmdStartScan, runID)
	if err != nil {
		t.Fatalf("build start command: %v", err)
	}

	observe := func(result classify.Result) scanstore.RecordObservationCmd {
		streamID, err := scanstore.StreamForDomain(result.Domain)
		if err != nil {
			t.Fatalf("stream for %s: %v", result.Domain, err)
		}

		basic, err := command.New(scanstore.CmdObserve, streamID)
		if err != nil {
			t.Fatalf("build observe command: %v", err)
		}

		return scanstore.RecordObservationCmd{BasicCommand: basic, RunID: runID.Get(), Result: result}
	}

	sc.When(scanstore.StartScanCmd{BasicCommand: startBasic, InputDomain: 2, Workers: 4}).
		Command(observe(workspaceResult("acme.com"))).
		Command(observe(failedResult("gone.example"))).
		Then("scan.started", "scan.domain_observed", "scan.domain_observed").
		ThenQueryFunc(func() (any, error) {
			view, found, err := metaengine.NewReader[scanstore.ObservationView](
				sc.System().MetaEngine(), scanstore.CollectionObservations).Get(ctx, "acme.com")
			if err != nil {
				return nil, err
			}

			if !found {
				return nil, errors.New("observation view for acme.com not projected yet")
			}

			return view, nil
		}, func(got any) error {
			view, ok := got.(scanstore.ObservationView)
			if !ok {
				t.Fatalf("unexpected view type %T", got)
			}

			if view.Domain != "acme.com" || view.LastRunID != runID.Get() || view.Observations != 1 {
				t.Errorf("view mismatch: %+v", view)
			}

			if len(view.Evidence) != 1 || view.Evidence[0].Kind != classify.EvidenceMXGoogle {
				t.Errorf("evidence mismatch: %+v", view.Evidence)
			}

			return nil
		})
}

// TestReplayRoundTrip is the core event-sourcing invariant: a fresh system
// booted on the same SQLite file rebuilds, purely from the journal, the
// exact results that were recorded — including per-run isolation and the
// last-observation-wins rule within a run.
func TestReplayRoundTrip(t *testing.T) {
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

	if err := recorder.StartScan(ctx, firstRun, len(firstResults), 0, 4); err != nil {
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

	secondRun := scanstore.NewRunID()
	changed := workspaceResult("acme.com")
	changed.Evidences = append(changed.Evidences, classify.Evidence{
		Kind:     classify.EvidenceSPFGoogle,
		Platform: classify.PlatformWorkspace,
		Weight:   classify.WeightWeak,
		Host:     "acme.com",
		Record:   "v=spf1 include:_spf.google.com ~all",
	})
	secondResults := []classify.Result{changed}

	if err := recorder.StartScan(ctx, secondRun, len(secondResults), 0, 4); err != nil {
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

	reader, err := scanstore.Boot(ctx, dsn)
	if err != nil {
		t.Fatalf("reboot store: %v", err)
	}

	defer func() { _ = reader.Close() }()

	latest, err := scanstore.ReplayJournal(ctx, reader, "")
	if err != nil {
		t.Fatalf("replay latest: %v", err)
	}

	if latest.RunID != secondRun.Get() {
		t.Errorf("latest run = %s, want %s", latest.RunID, secondRun.Get())
	}

	assertResults(t, latest.Results, secondResults)

	first, err := scanstore.ReplayJournal(ctx, reader, firstRun.Get())
	if err != nil {
		t.Fatalf("replay first run: %v", err)
	}

	assertResults(t, first.Results, firstResults)

	if len(first.Runs) != 2 {
		t.Fatalf("runs in journal = %d, want 2", len(first.Runs))
	}

	assertRunCounts(t, first.Runs, firstRun.Get(), classify.Count(firstResults))
	assertRunCounts(t, first.Runs, secondRun.Get(), classify.Count(secondResults))
}

func assertResults(t *testing.T, got, want []classify.Result) {
	t.Helper()

	if len(got) != len(want) {
		t.Fatalf("result count = %d, want %d (%+v)", len(got), len(want), got)
	}

	for i := range want {
		if got[i].Domain != want[i].Domain || got[i].Err != want[i].Err {
			t.Errorf("result %d = %+v, want %+v", i, got[i], want[i])
		}

		if len(got[i].Evidences) != len(want[i].Evidences) {
			t.Fatalf("result %d evidence count = %d, want %d", i, len(got[i].Evidences), len(want[i].Evidences))
		}

		for j := range want[i].Evidences {
			if got[i].Evidences[j] != want[i].Evidences[j] {
				t.Errorf("result %d evidence %d = %+v, want %+v", i, j, got[i].Evidences[j], want[i].Evidences[j])
			}
		}
	}
}

func assertRunCounts(t *testing.T, runs []scanstore.RunSummary, runID string, want classify.Counts) {
	t.Helper()

	for _, run := range runs {
		if run.RunID == runID {
			if !run.Completed {
				t.Errorf("run %s not marked completed", runID)
			}

			if run.Counts != want {
				t.Errorf("run %s counts = %+v, want %+v", runID, run.Counts, want)
			}

			return
		}
	}

	t.Errorf("run %s missing from replayed runs %v", runID, runs)
}
