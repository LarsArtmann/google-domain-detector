package cli

import (
	"bytes"
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	v5 "github.com/larsartmann/cmdguard/v5/pkg/cmdguard/v5"

	"github.com/larsartmann/google-domain-detector/internal/classify"
	"github.com/larsartmann/google-domain-detector/internal/resolve"
	"github.com/larsartmann/google-domain-detector/internal/scanstore"
)

// fakeRes is the same scripted resolver shape as scan's, reduced to what the
// CLI tests need.
type fakeRes struct {
	txt   map[string][]string
	mx    map[string][]*net.MX
	cname map[string]string
	ips   map[string][]net.IP
	ns    map[string][]*net.NS
}

func (f *fakeRes) LookupTXT(_ context.Context, host string) ([]string, error) {
	return f.txt[host], nil
}

func (f *fakeRes) LookupMX(_ context.Context, host string) ([]*net.MX, error) {
	return f.mx[host], nil
}

func (f *fakeRes) LookupCNAME(_ context.Context, host string) (string, error) {
	return f.cname[host], nil
}

func (f *fakeRes) LookupIP(_ context.Context, host string) ([]net.IP, error) {
	return f.ips[host], nil
}

func (f *fakeRes) LookupNS(_ context.Context, host string) ([]*net.NS, error) {
	return f.ns[host], nil
}

func (f *fakeRes) LookupExistence(_ context.Context, host string) (resolve.Existence, error) {
	return resolve.Exists, nil
}

func workspaceDomain() *fakeRes {
	return &fakeRes{
		txt: map[string][]string{"acme.com": {"v=spf1 include:_spf.google.com ~all"}},
		mx:  map[string][]*net.MX{"acme.com": {{Host: "smtp.google.com."}}},
	}
}

func run(t *testing.T, res *fakeRes, stdin string, args ...string) (string, string, error) {
	t.Helper()

	return runRes(t, res, stdin, args...)
}

func runRes(t *testing.T, res resolve.Resolver, stdin string, args ...string) (string, string, error) {
	t.Helper()

	var stdout, stderr bytes.Buffer

	d, err := BuildCLI(&stdout, &stderr, "test", "", res, strings.NewReader(stdin), false, false)
	if err != nil {
		t.Fatalf("BuildCLI failed: %v", err)
	}

	err = executeWithUsageContract(d, context.Background(), args)

	return stdout.String(), stderr.String(), err
}

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

// TestEnrichAppendsVerdictColumns pins the CSV enrichment mode (census T18):
// the input CSV streams through the event store's read model and comes back
// with every original column and row preserved, plus the normalized domain
// and derived verdicts. Unparseable cells and never-scanned domains keep
// their rows with explanatory error values — enrichment never drops rows.
func TestEnrichAppendsVerdictColumns(t *testing.T) {
	t.Parallel()

	store := filepath.Join(t.TempDir(), "scans.db")

	_, _, err := run(t, workspaceDomain(), "", "scan", "--no-ranges", "--event-store", store,
		"--format", "jsonl", "acme.com")
	if err != nil {
		t.Fatalf("seed scan failed: %v", err)
	}

	csvIn := "company,domain,tier\nAcme,https://acme.com/pricing,A\nBeta,not a domain,B\nGamma,missing.example,C\n"

	stdout, stderr, err := runRes(t, &fakeRes{}, csvIn,
		"enrich", "--event-store", store, "--input", "-")
	if err != nil {
		t.Fatalf("enrich failed: %v", err)
	}

	lines := strings.Split(strings.TrimRight(stdout, "\n"), "\n")
	if len(lines) != 4 {
		t.Fatalf("want header + 3 rows, got %d:\n%s", len(lines), stdout)
	}

	if lines[0] != "company,domain,tier,gdd_domain,workspace,gcp,error" {
		t.Errorf("header must append exactly the enrich columns, got: %s", lines[0])
	}

	if lines[1] != "Acme,https://acme.com/pricing,A,acme.com,certain,no," {
		t.Errorf("scanned row must carry derived verdicts, got: %s", lines[1])
	}

	if !strings.Contains(lines[2], "unparseable domain") || !strings.Contains(lines[2], "Beta") {
		t.Errorf("unparseable row must survive with an error value, got: %s", lines[2])
	}

	if !strings.Contains(lines[3], "not scanned") || !strings.Contains(lines[3], "Gamma") {
		t.Errorf("never-scanned row must survive with an error value, got: %s", lines[3])
	}

	if !strings.Contains(stderr, "3 rows enriched: 1 with recorded observations, 2 without") {
		t.Errorf("stderr must account the join, got:\n%s", stderr)
	}
}

// TestEnrichRejectsMissingStoreAndHeader pins the usage contract: no store
// or no recognizable domain column is exit 2, before any DNS or store work.
func TestEnrichRejectsMissingStoreAndHeader(t *testing.T) {
	t.Parallel()

	_, _, err := run(t, &fakeRes{}, "domain\nx.example\n", "enrich", "--input", "-")
	if code := v5.ExitCode(err); code != 2 {
		t.Errorf("enrich without --event-store must exit 2, got %d (%v)", code, err)
	}

	store := filepath.Join(t.TempDir(), "scans.db")

	if _, _, err2 := run(t, &fakeRes{}, "", "scan", "--no-ranges", "--event-store", store,
		"--format", "jsonl", "a.example"); err2 != nil {
		t.Fatalf("seed scan failed: %v", err2)
	}

	_, _, err = run(t, &fakeRes{}, "name,value\nfoo,bar\n", "enrich", "--event-store", store, "--input", "-")
	if code := v5.ExitCode(err); code != 2 {
		t.Errorf("enrich without a domain column must exit 2, got %d (%v)", code, err)
	}
}

// TestEnrichInputFileAndOpenError covers the file path of --input (the
// stdin '-' path is covered above): a real CSV file enriches like stdin,
// and a missing file fails with the path named instead of a usage error.
func TestEnrichInputFileAndOpenError(t *testing.T) {
	t.Parallel()

	store := filepath.Join(t.TempDir(), "scans.db")

	if _, _, err := run(t, &fakeRes{}, "", "scan", "--no-ranges", "--event-store", store,
		"--format", "jsonl", "acme.com"); err != nil {
		t.Fatalf("seed scan failed: %v", err)
	}

	csvPath := filepath.Join(t.TempDir(), "leads.csv")
	if err := os.WriteFile(csvPath, []byte("company,domain\nAcme,acme.com\n"), 0o600); err != nil {
		t.Fatalf("write input CSV: %v", err)
	}

	stdout, _, err := run(t, &fakeRes{}, "", "enrich", "--event-store", store, "--input", csvPath)
	if err != nil {
		t.Fatalf("enrich from file failed: %v", err)
	}

	if !strings.Contains(stdout, "company,domain,gdd_domain,workspace,gcp,error") {
		t.Errorf("enriched header missing appended columns, got: %q", stdout)
	}

	_, _, err = run(t, &fakeRes{}, "", "enrich", "--event-store", store,
		"--input", filepath.Join(t.TempDir(), "does-not-exist.csv"))
	if err == nil || !strings.Contains(err.Error(), "open --input") {
		t.Errorf("missing --input file must fail with the path named, got: %v", err)
	}
}

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

func TestUsageErrorsExitTwo(t *testing.T) {
	t.Parallel()

	_, _, err := run(t, workspaceDomain(), "", "scan", "--no-ranges")
	if err == nil {
		t.Fatal("empty non-TTY scan must fail")
	}

	if code := v5.ExitCode(err); code != 2 {
		t.Errorf("no-valid-domains exit code = %d, want 2 (usage error)", code)
	}

	_, _, err = run(t, workspaceDomain(), "", "replay")
	if err == nil {
		t.Fatal("replay without an event store must fail")
	}

	if code := v5.ExitCode(err); code != 2 {
		t.Errorf("replay-without-store exit code = %d, want 2 (usage error)", code)
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

func TestEnvPrefixResolution(t *testing.T) {
	// Not parallel: t.Setenv is incompatible with parallel subtests.
	store := filepath.Join(t.TempDir(), "events.db")
	t.Setenv("GDD_EVENT_STORE", store)

	if _, _, err := run(t, workspaceDomain(), "", "scan", "--no-ranges", "acme.com"); err != nil {
		t.Fatalf("scan failed: %v", err)
	}

	if _, err := os.Stat(store); err != nil {
		t.Errorf("GDD_EVENT_STORE did not resolve through the GDD_ env prefix: %v", err)
	}
}

func TestRecoveryMiddlewareRecoversPanics(t *testing.T) {
	t.Parallel()

	cli, err := v5.NewCLI(Name, "test", RootConfig{}, cliOptions("test", "")...)
	if err != nil {
		t.Fatal(err)
	}

	boom, boomErr := v5.NewCommand("boom", v5.NoFlags{},
		func(context.Context, *RootConfig, v5.NoFlags) error { panic("boom") },
		v5.WithShort("panics on purpose"),
	)
	b := v5.NewBuilder(cli)
	v5.Try(b, boom, boomErr)

	if err := b.Build(); err != nil {
		t.Fatal(err)
	}

	err = cli.ExecuteWithArgs(context.Background(), []string{"boom"})
	if err == nil {
		t.Fatal("a recovered panic must surface as an error, never as success")
	}

	if !strings.Contains(err.Error(), "boom") {
		t.Errorf("panic message lost by recovery middleware: %v", err)
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

func TestVersionCommand(t *testing.T) {
	t.Parallel()

	// version prints to the process stdout like the other battery
	// commands — the injected writer only sees handler output.
	if _, _, err := run(t, workspaceDomain(), "", "version"); err != nil {
		t.Fatalf("version failed: %v", err)
	}
}

func TestDescribeCommand(t *testing.T) {
	t.Parallel()

	// describe writes its JSON contract to the process stdout, not the
	// injected writer — asserting it exists and executes is the contract.
	_, _, err := run(t, workspaceDomain(), "", "describe")
	if err != nil {
		t.Fatalf("describe failed: %v", err)
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

func TestScanRecordsEventsAndReplayRebuildsSameResults(t *testing.T) {
	t.Parallel()

	store := filepath.Join(t.TempDir(), "scans.db")

	scanOut, scanErr, err := run(t, workspaceDomain(), "",
		"scan", "--no-ranges", "--format", "json", "--event-store", store,
		"acme.com", "other.example", "third.example")
	if err != nil {
		t.Fatalf("scan with event store failed: %v", err)
	}

	if !strings.Contains(scanErr, "recorded to "+store) {
		t.Errorf("stderr should name the run and store, got: %q", scanErr)
	}

	replayOut, replayErr, err := run(t, workspaceDomain(), "",
		"replay", "--event-store", store, "--format", "json")
	if err != nil {
		t.Fatalf("replay failed: %v", err)
	}

	if !strings.Contains(replayErr, "workspace certain 1") {
		t.Errorf("replay stderr should summarize the run, got: %q", replayErr)
	}

	// Scan preserves input order; replay preserves journal (completion)
	// order — the same facts may legitimately arrive in a different order.
	// What must never differ is the content.
	scanByDomain := jsonResultsByDomain(t, scanOut)
	replayByDomain := jsonResultsByDomain(t, replayOut)

	if len(scanByDomain) != 3 {
		t.Fatalf("scan produced %d results, want 3", len(scanByDomain))
	}

	for domain, scanned := range scanByDomain {
		replayed, ok := replayByDomain[domain]
		if !ok {
			t.Errorf("replay is missing %s", domain)

			continue
		}

		if !reflect.DeepEqual(scanned, replayed) {
			t.Errorf("replay result for %s differs:\nscan:   %q\nreplay: %q", domain, scanned, replayed)
		}
	}
}

// TestReplaySinceReportsVerdictFlips drives the delta query end to end:
// two scans of the same domains into one store with the resolver's answers
// changed in between, then `replay --since <run1>` must report exactly the
// flipped domains and the adoption/departure counts.
func TestReplaySinceReportsVerdictFlips(t *testing.T) {
	t.Parallel()

	store := filepath.Join(t.TempDir(), "scans.db")

	googleSPF := "v=spf1 include:_spf.google.com ~all"
	neutral := "v=spf1 include:mail.example ~all"

	res := &fakeRes{
		txt: map[string][]string{
			"adopter.example": {neutral},
			"leaver.example":  {googleSPF},
			"steady.example":  {neutral},
		},
	}

	_, firstErr, err := run(t, res, "",
		"scan", "--no-ranges", "--format", "json", "--event-store", store,
		"adopter.example", "leaver.example", "steady.example")
	if err != nil {
		t.Fatalf("first scan failed: %v", err)
	}

	firstRun := recordedRunID(t, firstErr)

	res.txt["adopter.example"] = []string{googleSPF}
	res.txt["leaver.example"] = []string{neutral}

	if _, _, err := run(t, res, "",
		"scan", "--no-ranges", "--format", "json", "--event-store", store,
		"adopter.example", "leaver.example", "steady.example"); err != nil {
		t.Fatalf("second scan failed: %v", err)
	}

	deltaOut, deltaErr, err := run(t, res, "",
		"replay", "--event-store", store, "--since", firstRun, "--format", "json")
	if err != nil {
		t.Fatalf("delta replay failed: %v", err)
	}

	byDomain := jsonResultsByDomain(t, deltaOut)
	for _, flipped := range []string{"adopter.example", "leaver.example"} {
		if _, ok := byDomain[flipped]; !ok {
			t.Errorf("delta output missing flipped domain %s; got %v", flipped, domainKeys(byDomain))
		}
	}

	if _, ok := byDomain["steady.example"]; ok {
		t.Errorf("steady domain must not appear in delta output; got %v", domainKeys(byDomain))
	}

	if !strings.Contains(deltaErr, "delta vs run "+firstRun) ||
		!strings.Contains(deltaErr, "workspace adopted 1 / left 1") {
		t.Errorf("delta stderr should carry the flip counts, got: %q", deltaErr)
	}

	streamOut, streamErr, err := run(t, res, "",
		"replay", "--event-store", store, "--since", firstRun, "--format", "jsonl")
	if err != nil {
		t.Fatalf("streaming delta replay failed: %v", err)
	}

	if lines := strings.Count(strings.TrimSpace(streamOut), "\n") + 1; lines != 2 {
		t.Errorf("streaming delta emitted %d lines, want exactly the 2 flips", lines)
	}

	if !strings.Contains(streamErr, "delta vs run "+firstRun+": 2 flips") {
		t.Errorf("streaming delta stderr should count 2 flips, got: %q", streamErr)
	}

	if _, _, err := run(t, res, "",
		"replay", "--event-store", store, "--since", "01NOPE", "--format", "json"); err == nil {
		t.Fatal("replay --since with an unknown run ID must fail")
	}
}

// nxForRes scripts oracle NXDOMAIN answers for named hosts on top of a
// base fakeRes; every other host keeps the base answers.
type nxForRes struct {
	fakeRes

	nx map[string]bool
}

func (f *nxForRes) LookupExistence(_ context.Context, host string) (resolve.Existence, error) {
	if f.nx[host] {
		return resolve.NotExists, nil
	}

	return resolve.Exists, nil
}

// TestReplayAuditExcludedFindsPoisonedSupersessions drives the poison check
// end to end: run 1 observes acme.com healthy and gone.example as a true
// NX; run 2 re-observes acme.com as NX and never refreshes gone.example.
// Excluding run 2 must name exactly acme.com; excluding run 1 must name
// exactly gone.example; excluding both must list both in first-seen order.
func TestReplayAuditExcludedFindsPoisonedSupersessions(t *testing.T) {
	t.Parallel()

	store, firstRun, secondRun := auditTestStore(t)

	auditOut, auditErr, err := runRes(t, workspaceDomain(), "",
		"replay", "--event-store", store, "--audit-excluded", secondRun, "--format", "json")
	if err != nil {
		t.Fatalf("audit excluding the poisoning run failed: %v", err)
	}

	if !strings.Contains(auditOut, "acme.com") || strings.Contains(auditOut, "gone.example") {
		t.Errorf("audit excluding the poisoning run should name exactly acme.com, got: %q", auditOut)
	}

	if !strings.Contains(auditErr, "supersession audit: 1 of 2 domain(s)") ||
		!strings.Contains(auditErr, secondRun+"=1") {
		t.Errorf("audit stderr should carry the poison counts, got: %q", auditErr)
	}

	staleOut, _, err := runRes(t, workspaceDomain(), "",
		"replay", "--event-store", store, "--audit-excluded", firstRun, "--format", "json")
	if err != nil {
		t.Fatalf("audit excluding the stale run failed: %v", err)
	}

	if !strings.Contains(staleOut, "gone.example") || strings.Contains(staleOut, "acme.com") {
		t.Errorf("audit excluding the stale run should name exactly gone.example, got: %q", staleOut)
	}

	bothOut, bothErr, err := runRes(t, workspaceDomain(), "",
		"replay", "--event-store", store, "--audit-excluded", firstRun+","+secondRun)
	if err != nil {
		t.Fatalf("audit excluding both runs failed: %v", err)
	}

	// Findings are ordered by first-seen JOURNAL order (completion order,
	// per the scan ordering contract) — which domain an racy oracle answers
	// first is not deterministic, so assert the attribution, not the order.
	acmeAt := strings.Index(bothOut, "acme.com")
	goneAt := strings.Index(bothOut, "gone.example")

	if acmeAt < 0 || goneAt < 0 {
		t.Errorf(
			"audit excluding both runs should name both poisoned domains, got: %q",
			bothOut,
		)
	}

	// The attribution IS deterministic: gone.example's last observation is
	// the first run's true NX, acme.com's is the second run's poison.
	attribution, _, err := runRes(t, workspaceDomain(), "",
		"replay", "--event-store", store, "--audit-excluded", firstRun+","+secondRun, "--format", "jsonl")
	if err != nil {
		t.Fatalf("streaming audit for attribution failed: %v", err)
	}

	for line := range strings.SplitSeq(strings.TrimSpace(attribution), "\n") {
		var finding struct {
			Domain string `json:"Domain"`
			Run    string `json:"Run"`
		}

		if err := json.Unmarshal([]byte(line), &finding); err != nil {
			t.Fatalf("audit line %q: %v", line, err)
		}

		want := map[string]string{"gone.example": firstRun, "acme.com": secondRun}[finding.Domain]
		if want == "" {
			t.Errorf("unexpected audit finding for %q", finding.Domain)

			continue
		}

		if finding.Run != want {
			t.Errorf("audit attribution for %s = %s, want %s", finding.Domain, finding.Run, want)
		}
	}

	if !strings.Contains(bothErr, "supersession audit: 2 of 2 domain(s)") {
		t.Errorf("auditing both runs should poison every domain, got stderr: %q", bothErr)
	}

	jsonlOut, _, err := runRes(t, workspaceDomain(), "",
		"replay", "--event-store", store, "--audit-excluded", firstRun+","+secondRun, "--format", "jsonl")
	if err != nil {
		t.Fatalf("streaming audit failed: %v", err)
	}

	if lines := strings.Count(strings.TrimSpace(jsonlOut), "\n") + 1; lines != 2 {
		t.Errorf("streaming audit emitted %d lines, want one per poisoned domain", lines)
	}
}

// TestReplayAuditExcludedRejectsBadInvocations pins the audit's usage
// contract: run selectors cannot combine with it (exit 2) and an unknown
// excluded run fails rather than silently reporting clean.
func TestReplayAuditExcludedRejectsBadInvocations(t *testing.T) {
	t.Parallel()

	store, firstRun, secondRun := auditTestStore(t)

	_, _, err := runRes(t, workspaceDomain(), "",
		"replay", "--event-store", store, "--audit-excluded", secondRun, "--run", firstRun)
	if err == nil {
		t.Fatal("--audit-excluded with --run must fail")
	}

	if code := v5.ExitCode(err); code != 2 {
		t.Errorf("--audit-excluded + --run exit code = %d, want 2 (usage error)", code)
	}

	if _, _, err = runRes(t, workspaceDomain(), "",
		"replay", "--event-store", store, "--audit-excluded", "01NOPE"); err == nil {
		t.Fatal("--audit-excluded with an unknown run ID must fail")
	}
}

// auditTestStore journals the two-run poison fixture through the real scan
// path and returns the store path plus both run IDs: run 1 observes
// acme.com healthy and gone.example as a true NX; run 2 re-observes
// acme.com as NX and never refreshes gone.example.
func auditTestStore(t *testing.T) (string, string, string) {
	t.Helper()

	store := filepath.Join(t.TempDir(), "scans.db")

	res1 := &nxForRes{fakeRes: *workspaceDomain(), nx: map[string]bool{"gone.example": true}}

	_, firstErr, err := runRes(t, res1, "",
		"scan", "--no-ranges", "--format", "json", "--event-store", store,
		"acme.com", "gone.example")
	if err != nil {
		t.Fatalf("first scan failed: %v", err)
	}

	firstRun := recordedRunID(t, firstErr)

	res2 := &nxForRes{fakeRes: *workspaceDomain(), nx: map[string]bool{"acme.com": true}}

	_, secondErr, err := runRes(t, res2, "",
		"scan", "--no-ranges", "--format", "json", "--event-store", store,
		"acme.com")
	if err != nil {
		t.Fatalf("second scan failed: %v", err)
	}

	return store, firstRun, recordedRunID(t, secondErr)
}

// recordedRunID extracts the run ID from a scan's stderr recording line.
func recordedRunID(t *testing.T, scanErr string) string {
	t.Helper()

	match := regexp.MustCompile(`run ([0-9A-Z]+) recorded to`).FindStringSubmatch(scanErr)
	if len(match) != 2 {
		t.Fatalf("scan stderr does not name a run ID: %q", scanErr)
	}

	return match[1]
}

func domainKeys(byDomain map[string]any) []string {
	keys := make([]string, 0, len(byDomain))
	for domain := range byDomain {
		keys = append(keys, domain)
	}

	return keys
}

// jsonResultsByDomain parses the scanner's JSON array output into a
// domain-keyed map of raw result objects.
func jsonResultsByDomain(t *testing.T, out string) map[string]any {
	t.Helper()

	var parsed []map[string]any

	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("parse JSON output: %v", err)
	}

	byDomain := make(map[string]any, len(parsed))
	for _, r := range parsed {
		byDomain[r["domain"].(string)] = r
	}

	return byDomain
}

func TestReplayRequiresEventStore(t *testing.T) {
	t.Parallel()

	_, _, err := run(t, workspaceDomain(), "", "replay")
	if err == nil {
		t.Fatal("replay without --event-store must fail")
	}
}

func TestReplayEmptyStoreFails(t *testing.T) {
	t.Parallel()

	store := filepath.Join(t.TempDir(), "missing.db")

	// Booting an empty store creates the file; replay must report no runs.
	_, _, err := run(t, workspaceDomain(), "", "replay", "--event-store", store)
	if err == nil {
		t.Fatal("replay of a store with no runs must fail")
	}
}

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

// showAfterScan retries show until the scan's projections catch up: the
// scan process journals synchronously, but the observation's read-model
// projection lands asynchronously, and an immediate show races it.
func showAfterScan(t *testing.T, res *fakeRes, args ...string) (string, string) {
	t.Helper()

	deadline := time.Now().Add(5 * time.Second)

	var (
		stdout, stderr string
		err            error
	)

	for {
		stdout, stderr, err = run(t, res, "", args...)
		if err == nil {
			return stdout, stderr
		}

		if time.Now().After(deadline) {
			t.Fatalf("show still failing after 5s (projection catch-up stalled?): %v", err)
		}

		time.Sleep(10 * time.Millisecond)
	}
}

func TestShowPrintsLatestObservation(t *testing.T) {
	t.Parallel()

	store := filepath.Join(t.TempDir(), "scans.db")

	if _, _, err := run(t, workspaceDomain(), "",
		"scan", "--no-ranges", "--event-store", store, "acme.com"); err != nil {
		t.Fatalf("scan failed: %v", err)
	}

	stdout, stderr := showAfterScan(t, workspaceDomain(),
		"show", "--event-store", store, "acme.com")

	if !strings.Contains(stdout, "acme.com") || !strings.Contains(stdout, "certain") {
		t.Errorf("show table should carry the domain and its verdict, got:\n%s", stdout)
	}

	if !strings.Contains(stderr, "1 observation(s)") {
		t.Errorf("show stderr should report the observation count, got: %q", stderr)
	}
}

func TestShowJSONCarriesEvidence(t *testing.T) {
	t.Parallel()

	store := filepath.Join(t.TempDir(), "scans.db")

	if _, _, err := run(t, workspaceDomain(), "",
		"scan", "--no-ranges", "--event-store", store, "acme.com"); err != nil {
		t.Fatalf("scan failed: %v", err)
	}

	stdout, _ := showAfterScan(t, workspaceDomain(),
		"show", "--event-store", store, "--format", "json", "acme.com")

	byDomain := jsonResultsByDomain(t, stdout)
	if _, ok := byDomain["acme.com"]; !ok {
		t.Fatalf("show JSON missing acme.com:\n%s", stdout)
	}
}

func TestShowUnknownDomainFails(t *testing.T) {
	t.Parallel()

	store := filepath.Join(t.TempDir(), "scans.db")

	if _, _, err := run(t, workspaceDomain(), "",
		"scan", "--no-ranges", "--event-store", store, "acme.com"); err != nil {
		t.Fatalf("scan failed: %v", err)
	}

	_, _, err := run(t, workspaceDomain(), "",
		"show", "--event-store", store, "never-scanned.example")
	if err == nil {
		t.Fatal("show of an unobserved domain must fail")
	}
}

func TestShowNeedsExactlyOneDomain(t *testing.T) {
	t.Parallel()

	_, _, err := run(t, workspaceDomain(), "", "show", "--event-store", "x.db")
	if err == nil {
		t.Fatal("show without a domain argument must fail")
	}
}

func TestRunsListsRecordedRuns(t *testing.T) {
	t.Parallel()

	store := filepath.Join(t.TempDir(), "scans.db")

	if _, _, err := run(t, workspaceDomain(), "",
		"scan", "--no-ranges", "--event-store", store, "acme.com"); err != nil {
		t.Fatalf("scan failed: %v", err)
	}

	stdout, stderr, err := run(t, workspaceDomain(), "", "runs", "--event-store", store)
	if err != nil {
		t.Fatalf("runs failed: %v", err)
	}

	if !strings.Contains(stdout, "completed") || !strings.Contains(stdout, "1") {
		t.Errorf("runs table should show the completed run with its tally, got:\n%s", stdout)
	}

	if !strings.Contains(stderr, "1 run(s)") {
		t.Errorf("runs stderr should report the run count, got: %q", stderr)
	}
}

func TestRunsEmptyStoreFails(t *testing.T) {
	t.Parallel()

	store := filepath.Join(t.TempDir(), "empty.db")

	if _, _, err := run(t, workspaceDomain(), "", "runs", "--event-store", store); err == nil {
		t.Fatal("runs on a store without runs must fail")
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

// TestScanCancelMidRunDrainsEventStore pins the G4 formality: Ctrl-C
// mid-scan still records every completed observation, CLOSES the run
// (CompleteScan survives the canceled context), and the partial run
// replays — the journal is a valid checkpoint to --resume from.
func TestScanCancelMidRunDrainsEventStore(t *testing.T) {
	t.Parallel()

	store := filepath.Join(t.TempDir(), "scans.db")

	var stderr bytes.Buffer

	stdout := &snapshotWriter{}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	d, err := BuildCLI(
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

	if err := d.ExecuteWithArgs(
		ctx,
		[]string{"scan", "--no-ranges", "--format", "jsonl", "--event-store", store, "fast1.com", "slow.com"},
	); err != nil {
		t.Fatalf("canceled scan must still finish cleanly: %v", err)
	}

	if !strings.Contains(stderr.String(), "run ") || !strings.Contains(stderr.String(), "recorded") {
		t.Fatalf("event store hint missing from stderr:\n%s", stderr.String())
	}

	reader, err := scanstore.Boot(context.Background(), store)
	if err != nil {
		t.Fatalf("boot store: %v", err)
	}

	defer func() { _ = reader.Close() }()

	replay, err := scanstore.ReplayJournal(context.Background(), reader, "")
	if err != nil {
		t.Fatalf("replay partial run: %v", err)
	}

	if len(replay.Results) != 2 {
		t.Fatalf(
			"partial run must journal both domains (completed + canceled), got %d: %+v",
			len(replay.Results),
			replay.Results,
		)
	}

	for _, run := range replay.Runs {
		if !run.Completed {
			t.Errorf("run must be completed despite cancellation: %+v", run)
		}

		if run.Counts.Scanned != 2 || run.Counts.Errors != 1 {
			t.Errorf("run counts = %+v, want 2 scanned / 1 error (canceled slow)", run.Counts)
		}
	}
}

// TestShowOpenErrorNamesStore covers the event-store open failure path:
// a missing store file must fail with the path named, not a usage error.
func TestShowOpenErrorNamesStore(t *testing.T) {
	t.Parallel()

	missing := filepath.Join(t.TempDir(), "absent.db")

	_, _, err := run(t, &fakeRes{}, "", "show", "--event-store", missing, "acme.com")
	if err == nil || !strings.Contains(err.Error(), missing) {
		t.Errorf("show on missing store must name the path, got: %v", err)
	}
}

// TestReplaySinceUnknownRunNamesID covers the --since unknown-run branch:
// a store WITH runs must reject an unknown baseline id by name.
func TestReplaySinceUnknownRunNamesID(t *testing.T) {
	t.Parallel()

	store := filepath.Join(t.TempDir(), "scans.db")

	if _, _, err := run(t, &fakeRes{}, "", "scan", "--no-ranges", "--event-store", store,
		"--format", "jsonl", "acme.com"); err != nil {
		t.Fatalf("seed scan failed: %v", err)
	}

	_, _, err := run(t, &fakeRes{}, "", "replay", "--event-store", store,
		"--since", "01NOPE", "--format", "jsonl")
	if err == nil || !strings.Contains(err.Error(), "01NOPE") {
		t.Errorf("replay --since unknown id must name the id, got: %v", err)
	}
}

// TestEnrichShortRowSurvives covers the short-row branch: a CSV record
// with fewer cells than the header keeps its row with an explanatory
// error cell instead of failing the run.
func TestEnrichShortRowSurvives(t *testing.T) {
	t.Parallel()

	store := filepath.Join(t.TempDir(), "scans.db")

	if _, _, err := run(t, &fakeRes{}, "", "scan", "--no-ranges", "--event-store", store,
		"--format", "jsonl", "acme.com"); err != nil {
		t.Fatalf("seed scan failed: %v", err)
	}

	stdout, _, err := run(t, &fakeRes{}, "company,domain\nAcme\n", "enrich",
		"--event-store", store, "--input", "-")
	if err != nil {
		t.Fatalf("enrich with a short row must not fail: %v", err)
	}

	if !strings.Contains(stdout, "short row (no domain column)") {
		t.Errorf("short row must carry the explanatory cell, got: %q", stdout)
	}
}

// TestReplaySinceEmptyStoreNamesPath covers the baseline ErrNoRuns branch:
// --since on a store with no runs must fail naming the store path.
func TestReplaySinceEmptyStoreNamesPath(t *testing.T) {
	t.Parallel()

	store := filepath.Join(t.TempDir(), "scans.db")

	if _, _, err := run(t, &fakeRes{}, "", "scan", "--no-ranges", "--event-store", store,
		"--format", "jsonl", "acme.com"); err != nil {
		t.Fatalf("seed scan failed: %v", err)
	}

	empty := filepath.Join(t.TempDir(), "empty.db")

	_, _, err := run(t, &fakeRes{}, "", "replay", "--event-store", empty,
		"--since", "01NOPE", "--format", "jsonl")
	if err == nil || !strings.Contains(err.Error(), empty) {
		t.Errorf("replay --since on an empty store must name the path, got: %v", err)
	}
}

// TestReplayBufferedFormatCoversRenderPath runs a non-jsonl replay so the
// buffered rebuild branch (renderResults) executes, not just streaming.
func TestReplayBufferedFormatCoversRenderPath(t *testing.T) {
	t.Parallel()

	store := filepath.Join(t.TempDir(), "scans.db")

	if _, _, err := run(t, workspaceDomain(), "", "scan", "--no-ranges", "--event-store", store,
		"--format", "jsonl", "acme.com"); err != nil {
		t.Fatalf("seed scan failed: %v", err)
	}

	stdout, _, err := run(t, &fakeRes{}, "", "replay", "--event-store", store, "--format", "csv")
	if err != nil {
		t.Fatalf("buffered replay failed: %v", err)
	}

	if !strings.Contains(stdout, "acme.com") {
		t.Errorf("csv replay must contain the domain, got: %q", stdout)
	}
}

// TestReplayUnknownRunNamesID covers the --run unknown-id branch.
func TestReplayUnknownRunNamesID(t *testing.T) {
	t.Parallel()

	store := filepath.Join(t.TempDir(), "scans.db")

	if _, _, err := run(t, &fakeRes{}, "", "scan", "--no-ranges", "--event-store", store,
		"--format", "jsonl", "acme.com"); err != nil {
		t.Fatalf("seed scan failed: %v", err)
	}

	_, _, err := run(t, &fakeRes{}, "", "replay", "--event-store", store,
		"--run", "01GHOST", "--format", "jsonl")
	if err == nil || !strings.Contains(err.Error(), "01GHOST") {
		t.Errorf("replay --run unknown id must name the id, got: %v", err)
	}
}

// TestReplayBufferedUnknownRunAndOpenError covers the buffered rebuild
// path's unknown-run branch and the store open failure.
func TestReplayBufferedUnknownRunAndOpenError(t *testing.T) {
	t.Parallel()

	store := filepath.Join(t.TempDir(), "scans.db")

	if _, _, err := run(t, &fakeRes{}, "", "scan", "--no-ranges", "--event-store", store,
		"--format", "jsonl", "acme.com"); err != nil {
		t.Fatalf("seed scan failed: %v", err)
	}

	_, _, err := run(t, &fakeRes{}, "", "replay", "--event-store", store, "--run", "01GHOST")
	if err == nil || !strings.Contains(err.Error(), "01GHOST") {
		t.Errorf("buffered replay --run unknown id must name the id, got: %v", err)
	}

	badStore := filepath.Join(t.TempDir(), "no-such-dir", "x.db")

	_, _, err = run(t, &fakeRes{}, "", "replay", "--event-store", badStore, "--format", "jsonl")
	if err == nil || !strings.Contains(err.Error(), badStore) {
		t.Errorf("replay on an unopenable store must name the path, got: %v", err)
	}
}

// TestRunsFlagBranches covers the runs command's usage and open errors.
func TestRunsFlagBranches(t *testing.T) {
	t.Parallel()

	_, _, err := run(t, &fakeRes{}, "", "runs")
	if code := v5.ExitCode(err); code != 2 {
		t.Errorf("runs without --event-store must exit 2, got %d (%v)", code, err)
	}

	badStore := filepath.Join(t.TempDir(), "no-such-dir", "x.db")

	_, _, err = run(t, &fakeRes{}, "", "runs", "--event-store", badStore)
	if err == nil || !strings.Contains(err.Error(), badStore) {
		t.Errorf("runs on an unopenable store must name the path, got: %v", err)
	}
}

// TestReplaySinceNewDomainIsNoFlip covers the baseline !seen branch: a
// domain first seen after the baseline run is not a verdict flip.
func TestReplaySinceNewDomainIsNoFlip(t *testing.T) {
	t.Parallel()

	store := filepath.Join(t.TempDir(), "scans.db")

	if _, _, err := run(t, &fakeRes{}, "", "scan", "--no-ranges", "--event-store", store,
		"--format", "jsonl", "acme.com"); err != nil {
		t.Fatalf("baseline scan failed: %v", err)
	}

	_, baselineStderr, err := run(t, &fakeRes{}, "", "scan", "--no-ranges", "--event-store", store,
		"--format", "jsonl", "acme.com")
	if err != nil {
		t.Fatalf("baseline scan failed: %v", err)
	}

	baselineRun := regexp.MustCompile(`run ([0-9A-Z]{10,32}) recorded`).FindStringSubmatch(baselineStderr)
	if baselineRun == nil {
		t.Fatalf("scan stderr must record the run id, got: %q", baselineStderr)
	}

	stdout, _, err := run(t, &fakeRes{}, "", "scan", "--no-ranges", "--event-store", store,
		"--format", "jsonl", "acme.com", "fresh.example")
	if err != nil {
		t.Fatalf("second scan failed: %v", err)
	}

	if !strings.Contains(stdout, "fresh.example") {
		t.Fatalf("second scan must cover the new domain, got: %q", stdout)
	}

	diffOut, _, err := run(t, &fakeRes{}, "", "replay", "--event-store", store,
		"--since", baselineRun[1], "--format", "jsonl")
	if err != nil {
		t.Fatalf("diff replay failed: %v", err)
	}

	if strings.Contains(diffOut, "fresh.example") {
		t.Errorf("a domain absent from the baseline is not a flip, but appeared: %q", diffOut)
	}
}
