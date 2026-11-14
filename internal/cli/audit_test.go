package cli

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	v5 "github.com/larsartmann/cmdguard/v5/pkg/cmdguard/v5"
)

// TestReplayAuditDataFormatsStayPure pins the audit table's data purity:
// the poison-count footer is a human decoration and must not leak into
// data formats (it did before the footer became table/markdown-only).
func TestReplayAuditDataFormatsStayPure(t *testing.T) {
	t.Parallel()

	store, _, secondRun := auditTestStore(t)

	stdout, _, err := runRes(t, workspaceDomain(), "",
		"replay", "--event-store", store, "--audit-excluded", secondRun, "--format", "csv")
	if err != nil {
		t.Fatalf("audit csv failed: %v", err)
	}

	for _, decoration := range []string{"poisoned of", "domain(s)", "Supersession Audit"} {
		if strings.Contains(stdout, decoration) {
			t.Errorf("audit csv must stay pure data: found %q in %q", decoration, stdout)
		}
	}

	if !strings.Contains(stdout, "acme.com") {
		t.Errorf("audit csv must still carry the finding row, got: %q", stdout)
	}
}

// TestReplayAuditExcludedAcceptsWaveNames pins the wave expansion: a wave
// name (wave:-prefixed or bare) expands to every run stamped with it, so
// the audit ritual names a wave instead of pasting ULIDs, and wave + run
// ID mix without double counting.
func TestReplayAuditExcludedAcceptsWaveNames(t *testing.T) {
	t.Parallel()

	const wave = "tranco1m-a"

	store, firstRun, secondRun := auditWaveTestStore(t, wave)

	attributions := func(t *testing.T, excluded string) {
		t.Helper()

		out, errOut, err := runRes(t, workspaceDomain(), "",
			"replay", "--event-store", store, "--audit-excluded", excluded, "--format", "jsonl")
		if err != nil {
			t.Fatalf("audit with %q failed: %v", excluded, err)
		}

		// Every domain is last observed by the excluded wave, and each
		// finding attributes to the run that poisoned it.
		seen := make(map[string]string)

		for line := range strings.SplitSeq(strings.TrimSpace(out), "\n") {
			var finding map[string]string

			if err := json.Unmarshal([]byte(line), &finding); err != nil {
				t.Fatalf("audit line %q: %v", line, err)
			}

			seen[finding["Domain"]] = finding["Run"]
		}

		want := map[string]string{"gone.example": firstRun, "acme.com": secondRun}
		for domain, run := range want {
			if seen[domain] != run {
				t.Errorf("attribution for %s = %q, want %q (excluded %q)", domain, seen[domain], run, excluded)
			}
		}

		if len(seen) != len(want) {
			t.Errorf("audit with %q found %v, want exactly %v", excluded, seen, want)
		}

		// Both wave runs appear exactly once in the per-run poison counts.
		for _, run := range []string{firstRun, secondRun} {
			if strings.Count(errOut, run+"=1") != 1 {
				t.Errorf("stderr must count %s exactly once, got: %q", run, errOut)
			}
		}
	}

	attributions(t, "wave:"+wave)
	attributions(t, wave)
	attributions(t, firstRun+","+secondRun)
	attributions(t, "wave:"+wave+","+firstRun)
}

// TestReplayAuditExcludedRejectsUnknownTokens pins the resolver's usage
// contract: an unknown wave name or an unknown bare token fails with exit
// 2 instead of silently reporting a clean audit.
func TestReplayAuditExcludedRejectsUnknownTokens(t *testing.T) {
	t.Parallel()

	store, _, _ := auditWaveTestStore(t, "tranco1m-a")

	for _, excluded := range []string{"wave:nope", "01NOPE"} {
		_, _, err := runRes(t, workspaceDomain(), "",
			"replay", "--event-store", store, "--audit-excluded", excluded)
		if err == nil {
			t.Fatalf("--audit-excluded %q must fail", excluded)
		}

		if code := v5.ExitCode(err); code != 2 {
			t.Errorf("--audit-excluded %q exit code = %d, want 2 (usage error)", excluded, code)
		}
	}
}

// auditWaveTestStore journals the two-run poison fixture under one wave
// name through the real scan path and returns the store path plus both
// run IDs: run 1 observes acme.com healthy and gone.example as a true NX;
// run 2 re-observes acme.com as NX and never refreshes gone.example.
func auditWaveTestStore(t *testing.T, wave string) (string, string, string) {
	t.Helper()

	store := filepath.Join(t.TempDir(), "scans.db")

	res1 := &nxForRes{fakeRes: *workspaceDomain(), nx: map[string]bool{"gone.example": true}}

	_, firstErr, err := runRes(t, res1, "",
		"scan", "--no-ranges", "--format", "json", "--event-store", store,
		"--wave", wave, "acme.com", "gone.example")
	if err != nil {
		t.Fatalf("first scan failed: %v", err)
	}

	firstRun := recordedRunID(t, firstErr)

	res2 := &nxForRes{fakeRes: *workspaceDomain(), nx: map[string]bool{"acme.com": true}}

	_, secondErr, err := runRes(t, res2, "",
		"scan", "--no-ranges", "--format", "json", "--event-store", store,
		"--wave", wave, "acme.com")
	if err != nil {
		t.Fatalf("second scan failed: %v", err)
	}

	return store, firstRun, recordedRunID(t, secondErr)
}
