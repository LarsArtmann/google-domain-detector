package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	v5 "github.com/larsartmann/cmdguard/v5/pkg/cmdguard/v5"
)

func TestScanArgumentsHappyPath(t *testing.T) {
	t.Parallel()

	stdout, _, err := run(t, workspaceDomain(), "", "scan", "--no-ranges", "acme.com")
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}

	if !strings.Contains(stdout, "acme.com") || !strings.Contains(stdout, "certain") {
		t.Errorf("table output incomplete:\n%s", stdout)
	}
}

func TestScanJSONFormat(t *testing.T) {
	t.Parallel()

	stdout, _, err := run(t, workspaceDomain(), "", "scan", "--no-ranges", "--format", "json", "acme.com")
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}

	if !strings.Contains(stdout, `"detected": true`) && !strings.Contains(stdout, `"detected":true`) {
		t.Errorf("json output missing verdicts:\n%s", stdout)
	}
}

func TestScanOnlyFilter(t *testing.T) {
	t.Parallel()

	stdout, _, err := run(t, workspaceDomain(), "", "scan", "--no-ranges", "--only", "gcp", "acme.com")
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}

	if strings.Contains(stdout, "acme.com") {
		t.Errorf("gcp filter must drop the workspace-only domain:\n%s", stdout)
	}
}

func TestScanReadsStdin(t *testing.T) {
	t.Parallel()

	stdout, _, err := run(t, workspaceDomain(), "acme.com\n", "scan", "--no-ranges")
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}

	if !strings.Contains(stdout, "acme.com") {
		t.Errorf("stdin domain missing from output:\n%s", stdout)
	}
}

func TestScanNoDomainsError(t *testing.T) {
	t.Parallel()

	_, _, err := run(t, workspaceDomain(), "", "scan", "--no-ranges")
	if err == nil || !strings.Contains(err.Error(), "no valid domains") {
		t.Errorf("want a no-domains error, got %v", err)
	}
}

func TestScanRejectsUnknownFormat(t *testing.T) {
	t.Parallel()

	_, _, err := run(t, workspaceDomain(), "", "scan", "--no-ranges", "--format", "bogus", "acme.com")
	if err == nil {
		t.Error("unknown format must fail the values validation")
	}
}

func TestScanAcceptsSerializationFormats(t *testing.T) {
	t.Parallel()

	for _, format := range []string{"yaml", "toml", "jsonl"} {
		stdout, _, err := run(t, workspaceDomain(), "", "scan", "--no-ranges", "--format", format, "acme.com")
		if err != nil {
			t.Errorf("%s: scan failed: %v", format, err)

			continue
		}

		if !strings.Contains(stdout, "acme.com") {
			t.Errorf("%s: domain missing from output:\n%s", format, stdout)
		}
	}
}

func TestScanJSONLOutputsOneObjectPerLine(t *testing.T) {
	t.Parallel()

	stdout, _, err := run(t, workspaceDomain(), "", "scan", "--no-ranges", "--format", "jsonl", "acme.com")
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}

	lines := strings.Split(strings.TrimRight(stdout, "\n"), "\n")
	if len(lines) != 1 {
		t.Fatalf("want 1 line for 1 domain, got %d:\n%s", len(lines), stdout)
	}

	var got map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &got); err != nil {
		t.Fatalf("jsonl line is not a JSON object: %v\n%s", err, lines[0])
	}

	for _, key := range []string{"domain", "workspace", "gcp", "evidence"} {
		if _, ok := got[key]; !ok {
			t.Errorf("jsonl line missing %q:\n%s", key, lines[0])
		}
	}
}

// delayedRes stalls the apex TXT lookup of chosen domains so later-input
// domains can complete first.
type delayedRes struct {
	*fakeRes

	delays map[string]time.Duration
}

func (d delayedRes) LookupTXT(ctx context.Context, host string) ([]string, error) {
	time.Sleep(d.delays[host])

	return d.fakeRes.LookupTXT(ctx, host)
}

// TestScanJSONLStreamsInCompletionOrder pins the streaming semantics: jsonl
// lines appear as results complete, so a slow first-input domain lands LAST,
// not first. A buffered renderer would print input order instead.
func TestScanJSONLStreamsInCompletionOrder(t *testing.T) {
	t.Parallel()

	res := &delayedRes{fakeRes: &fakeRes{}, delays: map[string]time.Duration{"slow.com": 150 * time.Millisecond}}

	stdout, _, err := runRes(t, res, "", "scan", "--no-ranges", "--format", "jsonl",
		"--workers", "3", "slow.com", "fast1.com", "fast2.com")
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}

	lines := strings.Split(strings.TrimRight(stdout, "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("want 3 lines, got %d:\n%s", len(lines), stdout)
	}

	var first, last map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &first); err != nil {
		t.Fatalf("first line is not JSON: %v\n%s", err, lines[0])
	}

	if err := json.Unmarshal([]byte(lines[2]), &last); err != nil {
		t.Fatalf("last line is not JSON: %v\n%s", err, lines[2])
	}

	if first["domain"] == "slow.com" {
		t.Errorf("slow domain streamed first; jsonl is not streaming:\n%s", stdout)
	}

	if last["domain"] != "slow.com" {
		t.Errorf("slow domain should complete last:\n%s", stdout)
	}
}

// hangRes blocks the TXT lookup of slow.com until its context dies, so the
// run can be canceled mid-flight while another domain already completed.
// When cancel is non-nil it fires once slow.com's TXT lookup begins AND
// (when wait is set) fast1.com's JSONL line is already visible in the run's
// stdout — both strictly after the event-store run record exists
// (attachEventStore precedes the worker pool). Waiting for the emitted line
// pins the completion order — the sink chains before the stream, so the
// result is classified before its line appears — so suite-load timing can
// neither pre-empt run-start nor flip the two lines the way the wall-clock
// AfterFunc and a bare lookup-start trigger could.
type hangRes struct {
	*fakeRes

	cancel context.CancelFunc
	wait   *snapshotWriter
	// holdAfterCancel keeps the worker inside the hung lookup for a grace
	// period AFTER the run died, so the feeder's parked select reliably
	// takes the ctx.Done branch instead of racing the worker back to the
	// queue (which would scan the next domain and blur the placeholder
	// accounting the caller is pinning).
	holdAfterCancel time.Duration
}

func (h hangRes) LookupTXT(ctx context.Context, host string) ([]string, error) {
	if host == "slow.com" {
		deadline := time.Now().Add(5 * time.Second)
		parentDead := false

		for !parentDead && h.wait != nil && !strings.Contains(h.wait.String(), "fast1.com") && time.Now().Before(deadline) {
			select {
			case <-ctx.Done():
				parentDead = true
			case <-time.After(500 * time.Microsecond):
			}
		}

		if h.cancel != nil {
			h.cancel()
		}

		<-ctx.Done()

		if h.holdAfterCancel > 0 {
			time.Sleep(h.holdAfterCancel)
		}

		return nil, ctx.Err()
	}

	return h.fakeRes.LookupTXT(ctx, host)
}

// snapshotWriter is a mutex-guarded buffer: the scan goroutines write while
// hangRes polls String from a worker goroutine — a bare bytes.Buffer would
// be a data race under -race.
type snapshotWriter struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (w *snapshotWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	return w.buf.Write(p)
}

func (w *snapshotWriter) String() string {
	w.mu.Lock()
	defer w.mu.Unlock()

	return w.buf.String()
}

// TestScanJSONLCancelMidStream proves cancellation stops the stream cleanly:
// the completed domain has its line, the hung domain reports the
// cancellation instead of hanging the run.
func TestScanJSONLCancelMidStream(t *testing.T) {
	t.Parallel()

	var stderr bytes.Buffer

	stdout := &snapshotWriter{}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	dispatcher, err := BuildCLI(
		stdout,
		&stderr,
		"test",
		"",
		&hangRes{fakeRes: &fakeRes{}, cancel: cancel, wait: stdout},
		strings.NewReader(""),
		false,
		false,
	)
	if err != nil {
		t.Fatalf("BuildCLI failed: %v", err)
	}

	// Ctrl-C sends an explicit cancellation (context.Canceled), not a
	// deadline; a parent deadline would classify as a per-domain timeout.
	// The cancel fires from inside hangRes only after fast1.com's line is
	// emitted — deterministic order under any suite load.
	err = dispatcher.ExecuteWithArgs(ctx, []string{"scan", "--no-ranges", "--format", "jsonl", "fast1.com", "slow.com"})
	if err != nil {
		t.Fatalf("canceled scan must still finish the report: %v", err)
	}

	lines := strings.Split(strings.TrimRight(stdout.String(), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("want exactly 2 jsonl lines, got %d:\n%s\nstderr:\n%s",
			len(lines), stdout.String(), stderr.String())
	}

	// The stream writes in COMPLETION order, and the deterministic cancel
	// fires the moment slow.com's lookup begins — under scheduler load the
	// canceled slow line can legitimately land before fast1's. Assert the
	// set: fast1 completes normally, slow reports its cancellation.
	if !strings.Contains(stdout.String(), `"domain":"fast1.com"`) {
		t.Errorf("completed domain must stream its line:\n%s", stdout.String())
	}

	slowLine := lines[0]
	if strings.Contains(lines[1], "slow.com") {
		slowLine = lines[1]
	}

	if !strings.Contains(slowLine, "slow.com") || !strings.Contains(slowLine, "canceled") {
		t.Errorf("hung domain must report its cancellation:\n%s", slowLine)
	}
}

func TestScanColorInvalidExitsTwo(t *testing.T) {
	t.Parallel()

	_, _, err := run(t, workspaceDomain(), "", "scan", "--color", "neon", "acme.com")

	exitErr, ok := errors.AsType[*v5.ExitError](err)
	if !ok || exitErr.ExitCode() != 2 {
		t.Fatalf("invalid --color must be a usage error (exit 2), got: %v", err)
	}
}

func TestScanColorNeverEmitsNoANSI(t *testing.T) {
	t.Parallel()

	stdout, _, err := run(t, workspaceDomain(), "",
		"scan", "--no-ranges", "--color", "never", "acme.com")
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}

	if strings.Contains(stdout, "\x1b[") {
		t.Errorf("--color never must suppress ANSI codes:\n%q", stdout)
	}
}
