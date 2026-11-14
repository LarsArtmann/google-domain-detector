package cli

import (
	"path/filepath"
	"strings"
	"testing"
	"time"
)

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
