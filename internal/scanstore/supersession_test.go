package scanstore_test

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/larsartmann/go-cqrs-lite/id/v4"
	"github.com/larsartmann/go-cqrs-lite/system/v4"

	"github.com/larsartmann/google-domain-detector/internal/classify"
	"github.com/larsartmann/google-domain-detector/internal/scanstore"
)

func nxOn(domain string) classify.Result {
	return classify.Result{Domain: domain, Err: "no such host", ErrKind: classify.ErrorNXDomain}
}

// auditJournal journals the poison fixture into a fresh store at dsn: run A
// observes adopt.example healthy and oldnx.example as NX; run B re-observes
// adopt.example as NX (the poisoned supersession shape) and adds
// later.example healthy, whose last observation is then also run B's, so
// excluding run B poisons it too: poison is provenance, not verdict.
// Journal first-seen domain order: adopt, oldnx, later. Returns both run IDs.
func auditJournal(t *testing.T, ctx context.Context, dsn string) (string, string) {
	t.Helper()

	writer, err := scanstore.Open(ctx, dsn)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}

	rec := scanstore.NewRecorder(writer)

	runA, runB := scanstore.NewRunID(), scanstore.NewRunID()

	steps := []struct {
		runID   id.StreamID
		wave    string
		results []classify.Result
	}{
		{runA, "audit-a", []classify.Result{workspaceResult("adopt.example"), nxOn("oldnx.example")}},
		{runB, "audit-b", []classify.Result{nxOn("adopt.example"), workspaceResult("later.example")}},
	}

	for _, step := range steps {
		if err := rec.StartScan(ctx, step.runID, "", step.wave, len(step.results), 0, 4); err != nil {
			t.Fatalf("start run %s: %v", step.wave, err)
		}

		for _, result := range step.results {
			if err := rec.Observe(ctx, step.runID, result); err != nil {
				t.Fatalf("observe %s in %s: %v", result.Domain, step.wave, err)
			}
		}
	}

	if err := writer.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}

	return runA.Get(), runB.Get()
}

// auditReader boots a journal-only reader on dsn, mirroring how the replay
// command reads: no projection host, so the audit pays no folding cost.
func auditReader(t *testing.T, ctx context.Context, dsn string) *system.System {
	t.Helper()

	reader, err := scanstore.Boot(ctx, dsn)
	if err != nil {
		t.Fatalf("boot reader: %v", err)
	}

	t.Cleanup(func() { _ = reader.Close() })

	return reader
}

func mustAudit(t *testing.T, ctx context.Context, sys *system.System, excluded []string) scanstore.SupersessionAudit {
	t.Helper()

	audit, err := scanstore.AuditSupersession(ctx, sys, excluded)
	if err != nil {
		t.Fatalf("audit excluded %v: %v", excluded, err)
	}

	return audit
}

func TestAuditSupersessionLastObservationWins(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	dsn := filepath.Join(t.TempDir(), "scans.db")

	runA, runB := auditJournal(t, ctx, dsn)
	reader := auditReader(t, ctx, dsn)

	audit := mustAudit(t, ctx, reader, []string{runB})
	if audit.TotalDomains != 3 {
		t.Errorf("TotalDomains = %d, want 3 (every domain the journal ever observed)", audit.TotalDomains)
	}

	want := []scanstore.SupersessionFinding{
		{Domain: "adopt.example", RunID: runB, ErrKind: classify.ErrorNXDomain, Err: "no such host"},
		{Domain: "later.example", RunID: runB},
	}

	if !reflect.DeepEqual(audit.Findings, want) {
		t.Errorf(
			"excluding the poisoning run = %+v, want adopt.example's NX plus later.example (both last seen in run B)",
			audit.Findings,
		)
	}

	audit = mustAudit(t, ctx, reader, []string{runA})
	want = []scanstore.SupersessionFinding{
		{Domain: "oldnx.example", RunID: runA, ErrKind: classify.ErrorNXDomain, Err: "no such host"},
	}

	if !reflect.DeepEqual(audit.Findings, want) {
		t.Errorf("excluding the stale run = %+v, want exactly oldnx.example's run-A NX", audit.Findings)
	}

	audit = mustAudit(t, ctx, reader, []string{runA, runB})
	want = []scanstore.SupersessionFinding{
		{Domain: "adopt.example", RunID: runB, ErrKind: classify.ErrorNXDomain, Err: "no such host"},
		{Domain: "oldnx.example", RunID: runA, ErrKind: classify.ErrorNXDomain, Err: "no such host"},
		{Domain: "later.example", RunID: runB},
	}

	if !reflect.DeepEqual(audit.Findings, want) {
		t.Errorf("excluding both runs = %+v, want all three in first-seen domain order", audit.Findings)
	}
}

func TestAuditSupersessionRejectsUnknownRuns(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	dsn := filepath.Join(t.TempDir(), "scans.db")

	runA, runB := auditJournal(t, ctx, dsn)
	reader := auditReader(t, ctx, dsn)

	if _, err := scanstore.AuditSupersession(ctx, reader, nil); !errors.Is(err, scanstore.ErrNoExcludedRuns) {
		t.Errorf("empty excluded set error = %v, want ErrNoExcludedRuns", err)
	}

	_, err := scanstore.AuditSupersession(ctx, reader, []string{"01NOPE"})
	if !errors.Is(err, scanstore.ErrUnknownRun) {
		t.Errorf("unknown run error = %v, want ErrUnknownRun", err)
	}

	_, err = scanstore.AuditSupersession(ctx, reader, []string{runA, "01NOPE", runB})
	if !errors.Is(err, scanstore.ErrUnknownRun) {
		t.Errorf("mixed known/unknown runs error = %v, want ErrUnknownRun naming the miss", err)
	}
}

func TestAuditSupersessionWithoutRuns(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	reader := auditReader(t, ctx, filepath.Join(t.TempDir(), "empty.db"))

	_, err := scanstore.AuditSupersession(ctx, reader, []string{"01SOMETHING"})
	if !errors.Is(err, scanstore.ErrNoRuns) {
		t.Errorf("empty-journal error = %v, want ErrNoRuns", err)
	}
}
