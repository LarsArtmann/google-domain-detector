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
		{"same ruleset stays silent", &scanstore.RunSummary{Ruleset: "r1"}, &scanstore.RunSummary{Ruleset: "r1"}, false},
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

	var exitErr *v5.ExitError
	if !errors.As(err, &exitErr) || exitErr.ExitCode() != exitCodeUsage {
		t.Fatalf("invalid --workers must be a usage error (exit 2), got: %v", err)
	}
}

func TestRunRejectsUnknownCommand(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer

	if err := Run(context.Background(), []string{"definitely-not-a-command"}, &stdout, &stderr, "test", ""); err == nil {
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

	if _, _, err := ranges.Load(context.Background(), ranges.Options{URL: server.URL, CachePath: cache, Now: time.Now}); err != nil {
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

	_, _, _, err := scanTargets(ScanFlags{}, nil, strings.NewReader(""), false, 0, nil, io.Discard)

	var exitErr *v5.ExitError
	if !errors.As(err, &exitErr) || exitErr.ExitCode() != exitCodeUsage {
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
	//nolint:paralleltest // t.Setenv mutates process env: must not race other tests

	cache := filepath.Join(t.TempDir(), "cloud.json")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(fixtureFeed))
	}))
	defer server.Close()

	if _, _, err := ranges.Load(context.Background(), ranges.Options{URL: server.URL, CachePath: cache, Now: time.Now}); err != nil {
		t.Fatalf("seed cache: %v", err)
	}

	t.Setenv("GDD_RANGES_URL", "http://127.0.0.1:1/unreachable")
	t.Setenv("GDD_RANGES_CACHE", cache)
	if err := checkRangesFeed(context.Background()); err != nil {
		t.Errorf("readable cache must pass the feed probe: %v", err)
	}
}

func TestDoctorCheckRangesFeedReportsFailure(t *testing.T) {
	//nolint:paralleltest // t.Setenv mutates process env: must not race other tests

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

	if _, err := os.Stat(filepath.Dir(cache)); err != nil {
		t.Errorf("cache parent dir should exist: %v", err)
	}
}
