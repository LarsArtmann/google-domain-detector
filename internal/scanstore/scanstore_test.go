package scanstore_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/larsartmann/go-cqrs-lite/command/v4"
	"github.com/larsartmann/go-cqrs-lite/id/v4"
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

func failedResult() classify.Result {
	return classify.Result{Domain: "gone.example", Err: "domain not found: nxdomain", ErrKind: classify.ErrorNXDomain}
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
		Command(observe(failedResult())).
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
	firstResults := []classify.Result{workspaceResult("acme.com"), failedResult()}

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

	if err := recorder.StartScan(ctx, secondRun, "", "", len(secondResults), 0, 4); err != nil {
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

// TestJournalOneHundredThousandEvents is the G4 benchmark: 100k observations
// append and replay within a sane budget, and the store size stays linear.
func TestJournalOneHundredThousandEvents(t *testing.T) {
	t.Parallel()

	// ~8-10 min under -race locally, and it blew CI's default 10-min go test
	// budget on 2-core runners twice (600s panic, goroutines idle). Perf
	// soaks are opt-in, not functional gates; measured numbers: AGENTS.md G4.
	if os.Getenv("GDD_SOAK") == "" {
		t.Skip("set GDD_SOAK=1 to run the 100k-event journal soak (exceeds CI's default test budget)")
	}

	ctx := context.Background()
	dsn := filepath.Join(t.TempDir(), "scans.db")

	writer, err := scanstore.Open(ctx, dsn)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}

	recorder := scanstore.NewRecorder(writer)

	runID := scanstore.NewRunID()

	const total = 100_000

	appendStart := time.Now()

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

	appendWall := time.Since(appendStart)

	reader, err := scanstore.Boot(ctx, dsn)
	if err != nil {
		t.Fatalf("boot reader: %v", err)
	}

	defer func() { _ = reader.Close() }()

	var replayed int

	replayStart := time.Now()

	if _, err := scanstore.StreamObservations(ctx, reader, "", func(classify.Result) error {
		replayed++

		return nil
	}); err != nil {
		t.Fatalf("stream replay: %v", err)
	}

	replayWall := time.Since(replayStart)

	size, err := os.Stat(dsn)
	if err != nil {
		t.Fatalf("stat store: %v", err)
	}

	var mem runtime.MemStats

	runtime.ReadMemStats(&mem)

	if replayed != total {
		t.Errorf("replayed %d of %d observations", replayed, total)
	}

	t.Logf("100k events: append %s (%.0f/s), replay %s (%.0f/s), db %d MiB, heap in use %d MiB",
		appendWall, float64(total)/appendWall.Seconds(),
		replayWall, float64(total)/replayWall.Seconds(),
		size.Size()/(1<<20), mem.HeapInuse/(1<<20))
}
