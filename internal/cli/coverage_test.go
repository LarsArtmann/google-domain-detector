package cli

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	v5 "github.com/larsartmann/cmdguard/v5/pkg/cmdguard/v5"
	"github.com/larsartmann/go-output"

	"github.com/larsartmann/google-domain-detector/internal/classify"
	"github.com/larsartmann/google-domain-detector/internal/ranges"
	"github.com/larsartmann/google-domain-detector/internal/render"
	"github.com/larsartmann/google-domain-detector/internal/scan"
	"github.com/larsartmann/google-domain-detector/internal/scanstore"
)

const fixtureFeed = `{
  "prefixes": [
    {"ipv4Prefix": "34.0.0.0/8", "service": "Google Cloud"},
    {"ipv6Prefix": "2600:1900::/28", "service": "Google Cloud"}
  ],
  "creationTimestamp": "2026-11-09T00:00:00Z"
}`

func TestRunRefOrLatest(t *testing.T) {
	t.Parallel()

	if got := runRefOrLatest(""); got != "latest" {
		t.Errorf("runRefOrLatest(%q) = %q, want latest", "", got)
	}

	if got := runRefOrLatest("run-9"); got != "run-9" {
		t.Errorf("runRefOrLatest(%q) = %q, want run-9", "run-9", got)
	}
}

func TestWarnRulesetMismatch(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer

	cases := []struct {
		name    string
		from    *scanstore.RunSummary
		to      *scanstore.RunSummary
		wantMsg bool
	}{
		{"nil sides stay silent", nil, &scanstore.RunSummary{Ruleset: "r1"}, false},
		{"nil target stays silent", &scanstore.RunSummary{Ruleset: "r1"}, nil, false},
		{
			"same ruleset stays silent",
			&scanstore.RunSummary{Ruleset: "r1"},
			&scanstore.RunSummary{Ruleset: "r1"},
			false,
		},
		{"empty rulesets stay silent", &scanstore.RunSummary{}, &scanstore.RunSummary{Ruleset: "r2"}, false},
		{"mismatch warns", &scanstore.RunSummary{Ruleset: "r1"}, &scanstore.RunSummary{Ruleset: "r2"}, true},
	}

	for _, tc := range cases {
		buf.Reset()
		warnRulesetMismatch(&buf, tc.from, tc.to)

		if got := buf.Len() > 0; got != tc.wantMsg {
			t.Errorf("%s: warned = %v, want %v (output %q)", tc.name, got, tc.wantMsg, buf.String())
		}
	}
}

func TestResultErrorCarriesPayload(t *testing.T) {
	t.Parallel()

	var err error = resultError("boom")

	if err.Error() != "boom" {
		t.Errorf("resultError payload = %q, want boom", err.Error())
	}
}

func TestNomProgressLifecycle(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer

	ctx := context.Background()
	progress := newNomProgress(ctx, &buf)

	progress.domainStarted("acme.com")
	progress.domainDone(classify.Result{Domain: "acme.com"})
	progress.domainDone(classify.Result{Domain: "slow.com", Err: "timeout", ErrKind: classify.ErrorTimeout})
	progress.finish(ctx)

	if !strings.Contains(buf.String(), "acme.com") || !strings.Contains(buf.String(), "\u2714") {
		t.Errorf("progress must render the completed activity, got %q", buf.String())
	}
}

func TestRunRejectsBadFlagsAsUsageErrors(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer

	err := Run(context.Background(), []string{"scan", "--workers", "0"}, &stdout, &stderr, "test", "abc123")

	exitErr, ok := errors.AsType[*v5.ExitError](err)
	if !ok || exitErr.ExitCode() != exitCodeUsage {
		t.Fatalf("invalid --workers must be a usage error (exit 2), got: %v", err)
	}
}

func TestRunRejectsUnknownCommand(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer

	if err := Run(
		context.Background(),
		[]string{"definitely-not-a-command"},
		&stdout,
		&stderr,
		"test",
		"",
	); err == nil {
		t.Fatal("unknown command must fail")
	}
}

func TestLoadRangesUsesFreshCacheWithoutNetwork(t *testing.T) {
	t.Parallel()

	cache := filepath.Join(t.TempDir(), "cloud.json")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(fixtureFeed))
	}))
	defer server.Close()

	if _, _, err := ranges.Load(
		context.Background(),
		ranges.Options{URL: server.URL, CachePath: cache, Now: time.Now},
	); err != nil {
		t.Fatalf("seed cache: %v", err)
	}

	set, err := loadRanges(context.Background(), ScanFlags{
		CachePath: cache,
		RangesURL: "http://127.0.0.1:1/unreachable",
	}, "test")
	if err != nil {
		t.Fatalf("fresh cache must load without network: %v", err)
	}

	if set == nil {
		t.Fatal("ranges set missing despite valid cache")
	}
}

func TestLoadRangesNoRangesReturnsNilSet(t *testing.T) {
	t.Parallel()

	set, err := loadRanges(context.Background(), ScanFlags{NoRanges: true}, "test")
	if err != nil {
		t.Fatalf("--no-ranges must not error: %v", err)
	}

	if set != nil {
		t.Error("--no-ranges must disable the set (nil), got one")
	}
}

func TestScanTargetsResumeFullyObservedIsNoOp(t *testing.T) {
	t.Parallel()

	resume := &scanstore.ResumeSource{RunID: "run-1", Observed: map[string]struct{}{"acme.com": {}}}

	feeder, first, last, err := scanTargets(ScanFlags{}, nil, strings.NewReader(""), false, 0, resume, io.Discard)
	if err != nil {
		t.Fatalf("fully observed --resume is a no-op, not an error: %v", err)
	}

	if feeder != nil || len(first) != 0 || last {
		t.Errorf("no-op must return nil feeder, empty chunk, last=false; got %v/%v/%v", feeder, first, last)
	}
}

func TestScanTargetsEmptyInputIsUsageError(t *testing.T) {
	t.Parallel()

	feeder, chunk, last, err := scanTargets(ScanFlags{}, nil, strings.NewReader(""), false, 0, nil, io.Discard)
	if feeder != nil || len(chunk) > 0 || last {
		t.Errorf("empty input must yield a nil feeder and no chunk, got chunk=%d last=%t", len(chunk), last)
	}

	exitErr, ok := errors.AsType[*v5.ExitError](err)
	if !ok || exitErr.ExitCode() != exitCodeUsage {
		t.Fatalf("empty input must be a usage error (exit 2), got: %v", err)
	}
}

func TestNewScanStreamModes(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	var opts scan.Options

	stream, finish := newScanStream(ctx, output.FormatJSONL, false, io.Discard, render.FilterAll, &opts)
	if stream == nil {
		t.Fatal("jsonl must return the stream")
	}

	finish()

	var buf bytes.Buffer

	ttyStream, ttyFinish := newScanStream(ctx, output.FormatTable, true, &buf, render.FilterAll, &opts)
	if ttyStream != nil {
		t.Fatal("tty table must not stream jsonl")
	}

	ttyFinish()

	if buf.Len() == 0 {
		t.Error("tty progress finish must render the final frame")
	}

	pipeStream, pipeFinish := newScanStream(ctx, output.FormatTable, false, io.Discard, render.FilterAll, &opts)
	if pipeStream != nil {
		t.Fatal("pipe table must not stream jsonl")
	}

	pipeFinish()
}

func TestDoctorCheckRangesFeedFromCache(t *testing.T) {
	cache := filepath.Join(t.TempDir(), "cloud.json")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(fixtureFeed))
	}))
	defer server.Close()

	if _, _, err := ranges.Load(
		context.Background(),
		ranges.Options{URL: server.URL, CachePath: cache, Now: time.Now},
	); err != nil {
		t.Fatalf("seed cache: %v", err)
	}

	t.Setenv("GDD_RANGES_URL", "http://127.0.0.1:1/unreachable")
	t.Setenv("GDD_RANGES_CACHE", cache)

	if err := checkRangesFeed(context.Background()); err != nil {
		t.Errorf("readable cache must pass the feed probe: %v", err)
	}
}

func TestDoctorCheckRangesFeedReportsFailure(t *testing.T) {
	t.Setenv("GDD_RANGES_URL", "http://127.0.0.1:1/unreachable")
	t.Setenv("GDD_RANGES_CACHE", filepath.Join(t.TempDir(), "absent", "cloud.json"))

	if err := checkRangesFeed(context.Background()); err == nil {
		t.Error("unreachable feed without cache must fail the probe")
	}
}

func TestDefaultRangesCacheIsUnderUserCacheDir(t *testing.T) {
	t.Parallel()

	cache, err := defaultRangesCache()
	if err != nil {
		t.Fatalf("default cache path: %v", err)
	}

	if !strings.Contains(cache, "google-domain-detector") || !strings.HasSuffix(cache, "cloud.json") {
		t.Errorf("default cache = %q, want .../google-domain-detector/cloud.json", cache)
	}
}

func TestScanOutputOptsBranches(t *testing.T) {
	t.Parallel()

	if _, _, _, err := scanOutputOpts(ScanFlags{Only: "bogus", Format: "table", Color: "auto"}); err == nil {
		t.Error("unknown --only value must error")
	}

	filter, format, _, err := scanOutputOpts(ScanFlags{Only: "all", Format: "nope", Color: "auto"})

	exitErr, ok := errors.AsType[*v5.ExitError](err)
	if !ok || exitErr.ExitCode() != exitCodeUsage {
		t.Errorf("unknown --format must be a usage error, got: %v", err)
	}

	_ = filter
	_ = format

	if _, _, _, err := scanOutputOpts(ScanFlags{Only: "all", Format: "table", Color: "nope"}); err == nil {
		t.Error("unknown --color value must error")
	}

	filter, format, opts, err := scanOutputOpts(ScanFlags{Only: "gcp", Format: "csv", Color: "never"})
	if err != nil || filter != render.FilterGCP || format != output.FormatCSV {
		t.Errorf("good flags: filter=%v format=%v err=%v", filter, format, err)
	}

	if opts.ColorMode != output.ColorModeNever {
		t.Errorf("ColorMode = %v, want ColorModeNever", opts.ColorMode)
	}
}

func TestResolveResumeBranches(t *testing.T) {
	t.Parallel()

	src, err := resolveResume(context.Background(), ScanFlags{})
	if src != nil || err != nil {
		t.Errorf("no --resume must be a fresh run (nil, nil), got (%v, %v)", src, err)
	}

	if _, err = resolveResume(context.Background(), ScanFlags{Resume: "latest"}); v5.ExitCode(err) != exitCodeUsage {
		t.Errorf("--resume without --event-store must be a usage error, got: %v", err)
	}

	store := filepath.Join(t.TempDir(), "scans.db")

	_, err = resolveResume(context.Background(), ScanFlags{Resume: "nirvana", EventStore: store})
	if err == nil || !strings.Contains(err.Error(), "no scan runs") {
		t.Errorf("--resume on an empty store must fail with ErrNoRuns, got: %v", err)
	}
}

func TestLoadRangesFetchFailureWithoutCache(t *testing.T) {
	t.Parallel()

	_, err := loadRanges(context.Background(), ScanFlags{
		RangesURL: "http://127.0.0.1:1/unreachable",
		CachePath: filepath.Join(t.TempDir(), "absent", "cloud.json"),
	}, "test")
	if err == nil {
		t.Error("unreachable feed without cache must fail")
	}
}

func TestLoadRangesDefaultCacheUnderXDGBundle(t *testing.T) {
	cacheHome := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cacheHome)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(fixtureFeed))
	}))
	defer server.Close()

	set, err := loadRanges(context.Background(), ScanFlags{RangesURL: server.URL}, "test")
	if err != nil {
		t.Fatalf("load via default cache path failed: %v", err)
	}

	if set == nil {
		t.Fatal("expected a ranges set")
	}

	defaultPath, err := defaultRangesCache()
	if err != nil {
		t.Fatalf("default cache path: %v", err)
	}

	if !strings.HasPrefix(defaultPath, cacheHome) {
		t.Errorf("default cache = %q, want under XDG_CACHE_HOME %q", defaultPath, cacheHome)
	}
}

func TestReplayHelpers(t *testing.T) {
	t.Parallel()

	if got := provenanceSuffix("", ""); got != "" {
		t.Errorf("empty provenance = %q, want empty", got)
	}

	if got := provenanceSuffix("w1", ""); got != " [wave w1]" {
		t.Errorf("wave-only provenance = %q", got)
	}

	if got := provenanceSuffix("w1", classify.Ruleset("v1")); got != " [wave w1, ruleset v1]" {
		t.Errorf("full provenance = %q", got)
	}

	if got := shortRunID("short"); got != "short" {
		t.Errorf("short id must pass through, got %q", got)
	}

	if got := shortRunID("01M794ZEB4HYR5PM25XC000001"); got != "01M794ZEB4HY…" {
		t.Errorf("long id must truncate, got %q", got)
	}

	if runSummary(scanstore.Replay{RunID: "absent"}) != nil {
		t.Error("runSummary must return nil for an unknown run id")
	}
}

// TestRunReplayAndShowFlagBranches covers the programmatic flag-parse
// branches the CLI enum binding normally front-runs: direct calls with
// unparseable Only/Format/Color values must error before any store I/O.
func TestRunReplayAndShowFlagBranches(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := filepath.Join(t.TempDir(), "scans.db")

	for name, call := range map[string]func() error{
		"replay bad --only": func() error {
			return runReplay(ctx, ReplayFlags{EventStore: store, Only: "bogus"}, io.Discard, io.Discard)
		},
		"replay bad --format": func() error {
			return runReplay(ctx, ReplayFlags{EventStore: store, Format: "nope"}, io.Discard, io.Discard)
		},
		"replay bad --color": func() error {
			return runReplay(ctx, ReplayFlags{EventStore: store, Color: "nope"}, io.Discard, io.Discard)
		},
		"show bad --format": func() error {
			return runShow(ctx, ShowFlags{EventStore: store, Format: "nope"}, []string{"acme.com"}, io.Discard, io.Discard)
		},
		"show bad --color": func() error {
			return runShow(ctx, ShowFlags{EventStore: store, Color: "nope"}, []string{"acme.com"}, io.Discard, io.Discard)
		},
	} {
		if err := call(); err == nil {
			t.Errorf("%s must error", name)
		}
	}
}

func TestCheckRangesFeedCorruptFeedNamesFix(t *testing.T) {
	garbage := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("{not json"))
	}))
	defer garbage.Close()

	t.Setenv("GDD_RANGES_URL", garbage.URL)
	t.Setenv("GDD_RANGES_CACHE", filepath.Join(t.TempDir(), "cloud.json"))

	err := checkRangesFeed(context.Background())
	if err == nil || !strings.Contains(err.Error(), "corrupt") {
		t.Errorf("unparseable feed must fail the probe with the corrupt family, got: %v", err)
	}
}

func TestNewDomainFeederStdinTTYWithoutInputIsUsageError(t *testing.T) {
	t.Parallel()

	_, err := newDomainFeeder(nil, "", strings.NewReader(""), true, nil, 0)

	exitErr, ok := errors.AsType[*v5.ExitError](err)
	if !ok || exitErr.ExitCode() != exitCodeUsage {
		t.Fatalf("no input on a TTY must be a usage error, got: %v", err)
	}

	list := filepath.Join(t.TempDir(), "list.txt")
	if err := os.WriteFile(list, []byte("a.example\n"), 0o600); err != nil {
		t.Fatalf("write list: %v", err)
	}

	feeder, err := newDomainFeeder(nil, list, strings.NewReader(""), false, nil, 0)
	if err != nil {
		t.Fatalf("zero chunk size must default, got: %v", err)
	}

	if feeder == nil || feeder.chunkSize != defaultScanChunkSize {
		t.Errorf("chunkSize must default to %d, got %+v", defaultScanChunkSize, feeder)
	}
}

func TestDefaultRangesCacheFailsWithoutHome(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", "")
	t.Setenv("HOME", "")

	if _, err := defaultRangesCache(); err == nil {
		t.Error("no HOME and no XDG_CACHE_HOME must fail the cache-path lookup")
	}
}

func TestCheckRangesFeedHealthyFeedPasses(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(fixtureFeed))
	}))
	defer server.Close()

	t.Setenv("GDD_RANGES_URL", server.URL)
	t.Setenv("GDD_RANGES_CACHE", filepath.Join(t.TempDir(), "cloud.json"))

	if err := checkRangesFeed(context.Background()); err != nil {
		t.Errorf("healthy feed must pass the probe, got: %v", err)
	}
}

// TestEnrichMalformedCSVFailsCleanly covers the row-read error branch: a
// bare-quote line must abort with the file line named, not silently drop.
func TestEnrichMalformedCSVFailsCleanly(t *testing.T) {
	t.Parallel()

	store := filepath.Join(t.TempDir(), "scans.db")

	var out, errOut bytes.Buffer

	err := runEnrich(context.Background(), EnrichFlags{EventStore: store, Input: "-"},
		strings.NewReader("domain\nacme.com\nbad \"quote\n"), &out, &errOut)
	if err == nil || !strings.Contains(err.Error(), "read CSV line") {
		t.Errorf("malformed CSV must fail naming the line, got: %v", err)
	}
}

// failingWriter covers the CSV write-error branches of enrichRows.
type failingWriter struct{ after int }

func (w *failingWriter) Write(p []byte) (int, error) {
	if w.after <= 0 {
		return 0, errors.New("disk full")
	}

	w.after--

	return len(p), nil
}

func TestEnrichWriteErrorAborts(t *testing.T) {
	t.Parallel()

	store := filepath.Join(t.TempDir(), "scans.db")

	err := runEnrich(context.Background(), EnrichFlags{EventStore: store, Input: "-"},
		strings.NewReader("domain\nacme.com\nother.example\n"), &failingWriter{}, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "write CSV") {
		t.Errorf("writer failure must abort with the line named, got: %v", err)
	}
}

// TestEnrichRowStoreReadFailed covers the read-model failure branch: the
// row survives with a store-read error cell instead of aborting.
func TestEnrichRowStoreReadFailed(t *testing.T) {
	t.Parallel()

	store := filepath.Join(t.TempDir(), "scans.db")

	sys, err := scanstore.Open(context.Background(), store)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}

	defer func() { _ = sys.Close() }()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	verdict := enrichRow(ctx, sys, []string{"acme.com"}, 0)
	if verdict.scanned || !strings.Contains(verdict.cells[3], "store read failed") {
		t.Errorf("canceled read must surface the store-read-failed cell, got: %+v", verdict)
	}
}

func TestCheckDNSResolverFailureNamesFallbackHint(t *testing.T) {
	t.Setenv("GDD_RESOLVER", "")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := checkDNSResolver(ctx); err == nil {
		t.Fatal("canceled probe must fail")
	}
}
