package cli

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	v5 "github.com/larsartmann/cmdguard/v5/pkg/cmdguard/v5"
)

// TestScanInterruptedSummaryCountsScannedOnly pins the interrupted-run
// accounting fix: a SIGINT'd run reports how many domains actually scanned,
// plus an explicit interrupted line for the input domains the cancel
// reached before their turn — instead of folding synthetic canceled
// placeholders into the "N domains" total (which made an interrupted run
// look like complete coverage).
func TestScanInterruptedSummaryCountsScannedOnly(t *testing.T) {
	t.Parallel()

	var stderr bytes.Buffer

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	dispatcher, err := BuildCLI(
		io.Discard,
		&stderr,
		"test",
		"",
		&hangRes{fakeRes: &fakeRes{}, cancel: cancel, holdAfterCancel: 250 * time.Millisecond},
		strings.NewReader(""),
		false,
		false,
	)
	if err != nil {
		t.Fatalf("BuildCLI failed: %v", err)
	}

	// One worker: it cancels the run the moment slow.com's TXT lookup
	// begins and stays pinned inside the lookup past the cancel, so
	// never.com — still queued behind it — is a placeholder, not a scan.
	// The summary must count slow.com only.
	err = dispatcher.ExecuteWithArgs(ctx, []string{
		"scan", "--no-ranges", "--workers", "1", "--format", "jsonl", "slow.com", "never.com",
	})
	if err != nil {
		t.Fatalf("canceled scan must still finish the report: %v", err)
	}

	if !strings.Contains(stderr.String(), "1 domains:") {
		t.Errorf("summary must count only the scanned domain, got:\n%s", stderr.String())
	}

	if !strings.Contains(
		stderr.String(),
		"interrupted: 1 input domains were never scanned (canceled before their turn)",
	) {
		t.Errorf("interrupted accounting line missing from stderr:\n%s", stderr.String())
	}
}

// ctxSlowRes holds every TXT lookup until its context dies or the delay
// elapses, honoring cancellation — the resolver shape a --max-runtime
// budget test needs (unlike delayedRes, which sleeps ctx-blind).
type ctxSlowRes struct {
	*fakeRes

	delay time.Duration
}

func (c ctxSlowRes) LookupTXT(ctx context.Context, host string) ([]string, error) {
	select {
	case <-ctx.Done():
		// Grace hold: keep the worker pinned past the deadline so the
		// feeder's parked select takes ctx.Done instead of racing the
		// worker back to the queue.
		time.Sleep(100 * time.Millisecond)

		return nil, ctx.Err()
	case <-time.After(c.delay):
	}

	return c.fakeRes.LookupTXT(ctx, host)
}

// TestScanMaxRuntimeBudgetExpiry pins E31's whole-run budget: when
// --max-runtime expires mid-run, the in-flight domain reports timeout, the
// unstarted one is interrupted (never scanned), and the summary counts
// only what really ran.
func TestScanMaxRuntimeBudgetExpiry(t *testing.T) {
	t.Parallel()

	stdout, stderr, err := runRes(t, &ctxSlowRes{fakeRes: &fakeRes{}, delay: 10 * time.Second}, "",
		"scan", "--no-ranges", "--workers", "1", "--timeout", "30s",
		"--max-runtime", "150ms", "--format", "jsonl", "slow.com", "never.com")
	if err != nil {
		t.Fatalf("budgeted scan must still finish the report: %v", err)
	}

	if !strings.Contains(stderr, "1 domains:") {
		t.Errorf("summary must count only the scanned domain, got:\n%s", stderr)
	}

	if !strings.Contains(stderr, "timeout 1") {
		t.Errorf("budget expiry must classify the in-flight domain as timeout, got:\n%s", stderr)
	}

	if !strings.Contains(stderr, "interrupted: 1 input domains were never scanned") {
		t.Errorf("unstarted domain must be accounted as interrupted, got:\n%s", stderr)
	}

	if strings.Contains(stdout, "never.com") {
		t.Errorf("never-started domain must not stream a line, got:\n%s", stdout)
	}
}

func TestScanRejectsUnknownFilter(t *testing.T) {
	t.Parallel()

	_, _, err := run(t, workspaceDomain(), "", "scan", "--no-ranges", "--only", "everything", "acme.com")
	if err == nil {
		t.Error("unknown filter must fail the values validation")
	}
}

func TestScanRejectsInvalidWorkers(t *testing.T) {
	t.Parallel()

	for _, workers := range []string{"0", "-1", "513"} {
		_, _, err := run(t, workspaceDomain(), "", "scan", "--no-ranges", "--workers", workers, "acme.com")
		if err == nil {
			t.Errorf("--workers %s must fail the validate tag", workers)
		}
	}
}

func TestScanFailureExitsOne(t *testing.T) {
	t.Parallel()

	broken := filepath.Join(t.TempDir(), "no-such-dir", "store.db")

	_, _, err := run(t, workspaceDomain(), "", "scan", "--no-ranges", "--event-store", broken, "acme.com")
	if err == nil {
		t.Fatal("scan against an unopenable event store must fail")
	}

	if code := v5.ExitCode(err); code != 1 {
		t.Errorf("scan-failure exit code = %d, want 1", code)
	}
}

func TestScanInputFile(t *testing.T) {
	t.Parallel()

	file := filepath.Join(t.TempDir(), "domains.csv")

	content := "Company,Website\nACME,https://www.acme.com\n"
	if err := os.WriteFile(file, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	stdout, _, err := run(t, workspaceDomain(), "", "scan", "--no-ranges", "--input", file)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}

	if !strings.Contains(stdout, "acme.com") {
		t.Errorf("CSV domain missing from output:\n%s", stdout)
	}
}

func TestScanSkippedEntriesReported(t *testing.T) {
	t.Parallel()

	_, stderr, err := run(t, workspaceDomain(), "", "scan", "--no-ranges", "acme.com", "not-a-domain")
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}

	if !strings.Contains(stderr, "skipped") {
		t.Errorf("skipped-entry warning missing:\n%s", stderr)
	}
}

func TestScanDomainTimeoutFlagIsUsed(t *testing.T) {
	t.Parallel()

	// Smoke: a short timeout flag is accepted and the run still completes
	// against an instant fake resolver.
	_, _, err := run(t, workspaceDomain(), "", "scan", "--no-ranges", "--timeout", "50ms", "acme.com")
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
}
