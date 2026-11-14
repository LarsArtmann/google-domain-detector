package cli

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

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
