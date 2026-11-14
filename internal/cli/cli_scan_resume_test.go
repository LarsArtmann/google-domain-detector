package cli

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/larsartmann/google-domain-detector/internal/classify"
	"github.com/larsartmann/google-domain-detector/internal/scanstore"
)

// TestScanInputParityAcrossSources pins 8.7: the same domain set yields the
// same report whether it arrives as arguments, a --input file, or piped
// stdin (input order is part of the table contract).
func TestScanInputParityAcrossSources(t *testing.T) {
	t.Parallel()

	res := workspaceDomain()
	args := []string{"scan", "--no-ranges", "acme.com", "beta.io", "gamma.net"}

	viaArgs, _, err := run(t, res, "", args...)
	if err != nil {
		t.Fatalf("args scan failed: %v", err)
	}

	list := filepath.Join(t.TempDir(), "domains.txt")
	if err := os.WriteFile(list, []byte("acme.com\nbeta.io\ngamma.net\n"), 0o600); err != nil {
		t.Fatalf("write list: %v", err)
	}

	viaFile, _, err := run(t, res, "", "scan", "--no-ranges", "--input", list)
	if err != nil {
		t.Fatalf("file scan failed: %v", err)
	}

	viaStdin, _, err := run(t, res, "acme.com\nbeta.io\ngamma.net\n", "scan", "--no-ranges")
	if err != nil {
		t.Fatalf("stdin scan failed: %v", err)
	}

	if viaArgs != viaFile || viaArgs != viaStdin {
		t.Errorf("input paths diverge:\nargs:\n%s\nfile:\n%s\nstdin:\n%s", viaArgs, viaFile, viaStdin)
	}
}

// TestScanMultiChunkRunRecordsAndReplays shrinks the chunk size and proves
// chunked scanning is invisible to the product contract: every domain is
// scanned exactly once, the event store records all of them, and replay
// rebuilds the full set. It calls runScan directly so the shrunken chunk
// size never touches shared state parallel tests could race on.
func TestScanMultiChunkRunRecordsAndReplays(t *testing.T) {
	t.Parallel()

	store := filepath.Join(t.TempDir(), "scans.db")

	domains := make([]string, 11) //nolint:makezero // presized, index-filled (input-order contract); 4 + 4 + 3 chunks
	for i := range domains {
		domains[i] = fmt.Sprintf("d%d.example", i)
	}

	var stdout, stderr bytes.Buffer

	flags := ScanFlags{
		Format:     "jsonl",
		Workers:    4,
		Timeout:    time.Second,
		Only:       "all",
		NoRanges:   true,
		EventStore: store,
	}

	if err := runScan(context.Background(), flags, domains, &stdout, &stderr,
		&fakeRes{}, strings.NewReader(""), false, false, "test", 4); err != nil {
		t.Fatalf("scan failed: %v", err)
	}

	scanOut := stdout.String()

	lines := strings.Split(strings.TrimRight(scanOut, "\n"), "\n")
	if len(lines) != len(domains) {
		t.Fatalf("want %d jsonl lines, got %d", len(domains), len(lines))
	}

	replayed, replayStderr, err := run(t, &fakeRes{}, "", "replay", "--event-store", store, "--format", "jsonl")
	if err != nil {
		t.Fatalf("replay failed: %v", err)
	}

	if got := strings.Count(replayed, `"domain"`); got != len(domains) {
		t.Errorf("replay rebuilt %d domains, want %d", got, len(domains))
	}

	if !strings.Contains(replayStderr, fmt.Sprintf("%d domains", len(domains))) {
		t.Errorf("replay summary missing the total:\n%s", replayStderr)
	}
}

// TestScanResumeSkipsObservedDomains pins 9.5: a --resume run scans only
// domains its source run never observed, journals exactly those, and links
// back to the source run in scan.started.
func TestScanResumeSkipsObservedDomains(t *testing.T) {
	t.Parallel()

	store := filepath.Join(t.TempDir(), "scans.db")

	_, firstStderr, err := run(t, &fakeRes{}, "", "scan", "--no-ranges", "--event-store", store,
		"--format", "jsonl", "a.example", "b.example", "c.example")
	if err != nil {
		t.Fatalf("first scan failed: %v", err)
	}

	firstRun := stderrRunID(t, firstStderr)

	stdout, resumeStderr, err := run(t, &fakeRes{}, "", "scan", "--no-ranges", "--event-store", store,
		"--resume", firstRun, "--format", "jsonl", "a.example", "b.example", "c.example", "d.example")
	if err != nil {
		t.Fatalf("resume scan failed: %v", err)
	}

	lines := strings.Split(strings.TrimRight(stdout, "\n"), "\n")
	if len(lines) != 1 || !strings.Contains(lines[0], "d.example") {
		t.Fatalf("resume must scan only the unobserved domain, got:\n%s", stdout)
	}

	wantAccounting := "resume: skipped 3 input domains already observed in run " + firstRun
	if !strings.Contains(resumeStderr, wantAccounting) {
		t.Errorf("resume must account its skips on stderr, want %q in:\n%s", wantAccounting, resumeStderr)
	}

	secondRun := stderrRunID(t, resumeStderr)
	if secondRun == firstRun {
		t.Fatal("resume must start a NEW run, not append to the source")
	}

	ctx := context.Background()

	sys, err := scanstore.Boot(ctx, store)
	if err != nil {
		t.Fatalf("boot store: %v", err)
	}

	defer func() { _ = sys.Close() }()

	replay, err := scanstore.ReplayJournal(ctx, sys, secondRun)
	if err != nil {
		t.Fatalf("replay resumed run: %v", err)
	}

	if len(replay.Results) != 1 || replay.Results[0].Domain != "d.example" {
		t.Errorf("resumed run journal = %v, want only d.example", replay.Results)
	}

	for _, r := range replay.Runs {
		if r.RunID == secondRun && r.ResumedFrom != firstRun {
			t.Errorf("resumed run links to %q, want %q", r.ResumedFrom, firstRun)
		}
	}
}

// TestScanResumeOfInterruptedRun pins 9.6: a run that started and recorded
// observations but never completed is a valid resume source — its observed
// domains are skipped, the rest are scanned.
func TestScanResumeOfInterruptedRun(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := filepath.Join(t.TempDir(), "scans.db")

	writer, err := scanstore.Open(ctx, store)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}

	recorder := scanstore.NewRecorder(writer)

	interrupted := scanstore.NewRunID()

	if err := recorder.StartScan(ctx, interrupted, "", "", 3, 0, 8); err != nil {
		t.Fatalf("start interrupted run: %v", err)
	}

	for _, name := range []string{"a.example", "b.example"} {
		if err := recorder.Observe(ctx, interrupted, classify.Result{Domain: name}); err != nil {
			t.Fatalf("observe %s: %v", name, err)
		}
	}

	// No CompleteScan: the run stays interrupted.

	if err := writer.GracefulClose(ctx); err != nil {
		t.Fatalf("close writer: %v", err)
	}

	stdout, _, err := run(t, &fakeRes{}, "", "scan", "--no-ranges", "--event-store", store,
		"--resume", interrupted.Get(), "--format", "jsonl", "a.example", "b.example", "c.example")
	if err != nil {
		t.Fatalf("resume of interrupted run failed: %v", err)
	}

	if lines := strings.Split(
		strings.TrimRight(stdout, "\n"),
		"\n",
	); len(lines) != 1 ||
		!strings.Contains(lines[0], "c.example") {
		t.Errorf("interrupted resume must scan only c.example, got:\n%s", stdout)
	}
}

// stderrRunID extracts the run ID from the scan command's stderr hint.
func stderrRunID(t *testing.T, stderr string) string {
	t.Helper()

	const marker = "run "

	fields := strings.Fields(strings.Split(stderr, " recorded")[0])
	if len(fields) == 0 {
		t.Fatalf("no run hint in stderr:\n%s", stderr)
	}

	return fields[len(fields)-1]
}
