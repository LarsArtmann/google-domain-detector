package cli

import (
	"bytes"
	"context"
	"encoding/json/v2"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"strings"
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

	d, err := BuildCLI(&stdout, &stderr, "test", "", res, strings.NewReader(stdin), false)
	if err != nil {
		t.Fatalf("BuildCLI failed: %v", err)
	}

	err = d.ExecuteWithArgs(context.Background(), args)

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
type hangRes struct {
	*fakeRes
}

func (h hangRes) LookupTXT(ctx context.Context, host string) ([]string, error) {
	if host == "slow.com" {
		<-ctx.Done()

		return nil, ctx.Err()
	}

	return h.fakeRes.LookupTXT(ctx, host)
}

// TestScanJSONLCancelMidStream proves cancellation stops the stream cleanly:
// the completed domain has its line, the hung domain reports the
// cancellation instead of hanging the run.
func TestScanJSONLCancelMidStream(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer

	d, err := BuildCLI(&stdout, &stderr, "test", "", &hangRes{&fakeRes{}}, strings.NewReader(""), false)
	if err != nil {
		t.Fatalf("BuildCLI failed: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Ctrl-C sends an explicit cancellation (context.Canceled), not a
	// deadline; a parent deadline would classify as a per-domain timeout.
	time.AfterFunc(100*time.Millisecond, cancel)

	err = d.ExecuteWithArgs(ctx, []string{"scan", "--no-ranges", "--format", "jsonl", "fast1.com", "slow.com"})
	if err != nil {
		t.Fatalf("canceled scan must still finish the report: %v", err)
	}

	lines := strings.Split(strings.TrimRight(stdout.String(), "\n"), "\n")
	if len(lines) != 2 || !strings.Contains(lines[0], "fast1.com") {
		t.Fatalf("want fast1 line then canceled slow line, got %d lines:\n%s\nstderr:\n%s",
			len(lines), stdout.String(), stderr.String())
	}

	if !strings.Contains(lines[1], "slow.com") || !strings.Contains(lines[1], "canceled") {
		t.Errorf("hung domain must report its cancellation:\n%s", lines[1])
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
// rebuilds the full set.
func TestScanMultiChunkRunRecordsAndReplays(t *testing.T) {
	t.Parallel()

	prev := scanChunkSize
	scanChunkSize = 4

	t.Cleanup(func() { scanChunkSize = prev })

	store := filepath.Join(t.TempDir(), "scans.db")

	domains := make([]string, 11) // 4 + 4 + 3 chunks
	for i := range domains {
		domains[i] = fmt.Sprintf("d%d.example", i)
	}

	stdout, _, err := run(
		t,
		&fakeRes{},
		"",
		append([]string{"scan", "--no-ranges", "--format", "jsonl", "--event-store", store}, domains...)...)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}

	lines := strings.Split(strings.TrimRight(stdout, "\n"), "\n")
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
