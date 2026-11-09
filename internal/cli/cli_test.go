package cli

import (
	"bytes"
	"context"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
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

	var stdout, stderr bytes.Buffer
	d, err := BuildCLI(&stdout, &stderr, "test", res, strings.NewReader(stdin), false)
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

	_, _, err := run(t, workspaceDomain(), "", "scan", "--no-ranges", "--format", "yaml", "acme.com")
	if err == nil {
		t.Error("yaml is not a registered table format and must fail the values validation")
	}
}

func TestScanRejectsUnknownFilter(t *testing.T) {
	t.Parallel()

	_, _, err := run(t, workspaceDomain(), "", "scan", "--no-ranges", "--only", "everything", "acme.com")
	if err == nil {
		t.Error("unknown filter must fail the values validation")
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
