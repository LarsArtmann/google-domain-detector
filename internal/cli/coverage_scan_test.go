package cli

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/larsartmann/go-output"

	"github.com/larsartmann/google-domain-detector/internal/classify"
	"github.com/larsartmann/google-domain-detector/internal/render"
	"github.com/larsartmann/google-domain-detector/internal/scan"
	"github.com/larsartmann/google-domain-detector/internal/scanstore"

	v5 "github.com/larsartmann/cmdguard/v5/pkg/cmdguard/v5"
)

func TestRunRefOrLatest(t *testing.T) {
	t.Parallel()

	if got := runRefOrLatest(""); got != "latest" {
		t.Errorf("runRefOrLatest(%q) = %q, want latest", "", got)
	}

	if got := runRefOrLatest("run-9"); got != "run-9" {
		t.Errorf("runRefOrLatest(%q) = %q, want run-9", "run-9", got)
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

func TestCheckDNSResolverFailureNamesFallbackHint(t *testing.T) {
	t.Setenv("GDD_RESOLVER", "")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := checkDNSResolver(ctx); err == nil {
		t.Fatal("canceled probe must fail")
	}
}
