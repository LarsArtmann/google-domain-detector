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
	// per the scan ordering contract) — which domain a racy oracle answers
	// first is not deterministic, so assert the attribution, not the order.
	if !strings.Contains(bothOut, "acme.com") || !strings.Contains(bothOut, "gone.example") {
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
		// The audit DTO's JSON keys are capitalized (render DTO contract);
		// decode into a map so the test needs no tagged struct.
		var finding map[string]any

		if err := json.Unmarshal([]byte(line), &finding); err != nil {
			t.Fatalf("audit line %q: %v", line, err)
		}

		domain, _ := finding["Domain"].(string)
		runID, _ := finding["Run"].(string)

		want := map[string]string{"gone.example": firstRun, "acme.com": secondRun}[domain]
		if want == "" {
			t.Errorf("unexpected audit finding for %q", domain)

			continue
		}

		if runID != want {
			t.Errorf("audit attribution for %s = %s, want %s", domain, runID, want)
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
