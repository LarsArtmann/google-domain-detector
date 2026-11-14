package cli

import (
	"context"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	v5 "github.com/larsartmann/cmdguard/v5/pkg/cmdguard/v5"

	"github.com/larsartmann/google-domain-detector/internal/resolve"
)

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

// TestShowOpenErrorNamesStore covers the event-store open failure path:
// a missing store file must fail with the path named, not a usage error.
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
