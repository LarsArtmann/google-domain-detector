package scanstore_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/larsartmann/google-domain-detector/internal/classify"
	"github.com/larsartmann/google-domain-detector/internal/scanstore"
)

func plainResult(domainName string) classify.Result {
	return classify.Result{Domain: domainName}
}

func spfResult(domainName string) classify.Result {
	return classify.Result{
		Domain: domainName,
		Evidences: []classify.Evidence{
			{
				Kind:     classify.EvidenceSPFGoogle,
				Platform: classify.PlatformWorkspace,
				Weight:   classify.WeightWeak,
				Host:     domainName,
				Record:   "v=spf1 include:_spf.google.com ~all",
			},
		},
	}
}

func gcpResult(domainName string) classify.Result {
	return classify.Result{
		Domain: domainName,
		Evidences: []classify.Evidence{
			{
				Kind:     classify.EvidenceIPGCPRange,
				Platform: classify.PlatformGCP,
				Weight:   classify.WeightStrong,
				Host:     domainName,
				Record:   "34.0.0.1",
			},
		},
	}
}

func erroredResult(domainName string, kind classify.ErrorKind) classify.Result {
	return classify.Result{Domain: domainName, Err: "boom", ErrKind: kind}
}

// TestFlipDiffCensusGuards pins the delta semantics: adoption and departure
// per platform flip, confidence upgrades count as flips, errored lookups are
// unknowns that can NEVER read as a departure, and one-sided domains are not
// flips at all.
func TestFlipDiffCensusGuards(t *testing.T) {
	t.Parallel()

	from := []classify.Result{
		plainResult("adopter.example"),   // → workspace
		workspaceResult("leaver.example"), // → plain
		plainResult("gcp-adopter.example"),
		spfResult("upgrader.example"), // likely → certain
		workspaceResult("flaky.example"), // answers, then times out
		plainResult("only-from.example"), // not rescanned
	}

	to := []classify.Result{
		workspaceResult("adopter.example"),
		plainResult("leaver.example"),
		gcpResult("gcp-adopter.example"),
		workspaceResult("upgrader.example"),
		erroredResult("flaky.example", classify.ErrorTimeout),
		plainResult("only-to.example"), // newly covered
	}

	flips := scanstore.FlipDiff(from, to)

	got := map[string]bool{}
	for _, f := range flips {
		got[f.Domain] = true
	}

	for _, want := range []string{"adopter.example", "leaver.example", "gcp-adopter.example", "upgrader.example"} {
		if !got[want] {
			t.Errorf("flip %s missing; got %v", want, flipDomains(flips))
		}
	}

	for _, never := range []string{"flaky.example", "only-from.example", "only-to.example"} {
		if got[never] {
			t.Errorf("domain %s must never register as a flip (errored/one-sided)", never)
		}
	}

	counts := scanstore.CountFlips(flips)
	if counts.WorkspaceAdopted != 1 || counts.WorkspaceLeft != 1 || counts.GCPAdopted != 1 || counts.GCPLeft != 0 {
		t.Errorf("counts = %+v, want ws adopted/left 1/1, gcp adopted/left 1/0", counts)
	}
}

func flipDomains(flips []scanstore.Delta) []string {
	names := make([]string, 0, len(flips))
	for _, f := range flips {
		names = append(names, f.Domain)
	}

	return names
}

// TestFlipDiffAcrossRuns journals two runs through the real store, replays
// both, and diffs them: the delta must come out of the journal alone, the
// way the census delta query consumes it.
func TestFlipDiffAcrossRuns(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	dsn := filepath.Join(t.TempDir(), "scans.db")

	writer, err := scanstore.Open(ctx, dsn)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}

	recorder := scanstore.NewRecorder(writer)

	firstRun := scanstore.NewRunID()
	firstResults := []classify.Result{
		plainResult("adopter.example"),
		workspaceResult("leaver.example"),
		plainResult("steady.example"),
	}

	if err := recorder.StartScan(ctx, firstRun, "", "wave-a", len(firstResults), 0, 2); err != nil {
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
	secondResults := []classify.Result{
		workspaceResult("adopter.example"),
		plainResult("leaver.example"),
		plainResult("steady.example"),
	}

	if err := recorder.StartScan(ctx, secondRun, firstRun.Get(), "wave-a", len(secondResults), 0, 2); err != nil {
		t.Fatalf("start second run: %v", err)
	}

	for _, result := range secondResults {
		if err := recorder.Observe(ctx, secondRun, result); err != nil {
			t.Fatalf("observe: %v", err)
		}
	}

	if err := recorder.CompleteScan(ctx, secondRun, time.Minute, classify.Count(secondResults)); err != nil {
		t.Fatalf("complete second run: %v", err)
	}

	if err := writer.GracefulClose(ctx); err != nil {
		t.Fatalf("close writer: %v", err)
	}

	sys, err := scanstore.Boot(ctx, dsn)
	if err != nil {
		t.Fatalf("boot for replay: %v", err)
	}

	defer func() { _ = sys.Close() }()

	fromReplay, err := scanstore.ReplayJournal(ctx, sys, firstRun.Get())
	if err != nil {
		t.Fatalf("replay first run: %v", err)
	}

	toReplay, err := scanstore.ReplayJournal(ctx, sys, secondRun.Get())
	if err != nil {
		t.Fatalf("replay second run: %v", err)
	}

	flips := scanstore.FlipDiff(fromReplay.Results, toReplay.Results)
	if len(flips) != 2 {
		t.Fatalf("journal-derived flips = %v, want exactly adopter+leaver", flipDomains(flips))
	}

	counts := scanstore.CountFlips(flips)
	if counts.WorkspaceAdopted != 1 || counts.WorkspaceLeft != 1 {
		t.Errorf("journal-derived counts = %+v, want ws adopted/left 1/1", counts)
	}
}
