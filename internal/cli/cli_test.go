package cli

import (
	"bytes"
	"context"
	"encoding/json/v2"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	v5 "github.com/larsartmann/cmdguard/v5/pkg/cmdguard/v5"

	"github.com/larsartmann/google-domain-detector/internal/resolve"
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
