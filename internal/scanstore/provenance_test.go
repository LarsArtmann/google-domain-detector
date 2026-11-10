package scanstore_test

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/larsartmann/go-cqrs-lite/event/v4"
	"github.com/larsartmann/go-cqrs-lite/id/v4"

	"github.com/larsartmann/google-domain-detector/internal/classify"
	"github.com/larsartmann/google-domain-detector/internal/scanstore"
)

// readAllEvents drains the journal in store order via whichever read
// interface the store exposes (seeking or full read).
func readAllEvents(ctx context.Context, t *testing.T, store event.Store) []event.Event {
	t.Helper()

	if seekable, ok := store.(event.SeekableJournal); ok {
		var out []event.Event

		var after id.EventID

		for {
			batch, err := seekable.ReadFrom(ctx, after, 1000)
			if err != nil {
				t.Fatalf("read journal batch: %v", err)
			}

			if len(batch) == 0 {
				return out
			}

			out = append(out, batch...)
			after = batch[len(batch)-1].ID()
		}
	}

	journal, ok := store.(event.Journal)
	if !ok {
		t.Fatal("event store supports neither seeking nor journal reads")
	}

	events, err := journal.ReadAll(ctx)
	if err != nil {
		t.Fatalf("read journal: %v", err)
	}

	return events
}

// TestProvenanceStamps pins the census guards: every run journals its wave
// ID and the frozen detection ruleset, and every observation carries the
// ruleset as evidence provenance — the fields every cross-run delta query
// will key on.
func TestProvenanceStamps(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	dsn := filepath.Join(t.TempDir(), "provenance.db")

	writer, err := scanstore.Open(ctx, dsn)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}

	runID := scanstore.NewRunID()

	recorder := scanstore.NewRecorder(writer)
	if err := recorder.StartScan(ctx, runID, "", "tranco1m-a", 2, 0, 4); err != nil {
		t.Fatalf("start scan: %v", err)
	}

	if err := recorder.Observe(ctx, runID, workspaceResult("acme.com")); err != nil {
		t.Fatalf("observe acme.com: %v", err)
	}

	if err := recorder.CompleteScan(ctx, runID, 0, classify.Counts{Scanned: 1}); err != nil {
		t.Fatalf("complete scan: %v", err)
	}

	events := readAllEvents(ctx, t, writer.EventStore())

	var startedRuleset, observedRuleset classify.Ruleset

	for _, evt := range events {
		switch evt.Type() {
		case "scan.started":
			p, err := event.DecodePayloadAuto[scanstore.ScanStartedPayload](evt)
			if err != nil {
				t.Fatalf("decode scan.started: %v", err)
			}

			if p.Wave != "tranco1m-a" {
				t.Errorf("scan.started wave = %q, want %q", p.Wave, "tranco1m-a")
			}

			startedRuleset = p.Ruleset
		case "scan.domain_observed":
			p, err := event.DecodePayloadAuto[scanstore.DomainObservedPayload](evt)
			if err != nil {
				t.Fatalf("decode scan.domain_observed: %v", err)
			}

			observedRuleset = p.Ruleset
		}
	}

	if err := writer.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}

	if startedRuleset != classify.CurrentRuleset {
		t.Errorf("scan.started ruleset = %q, want %q", startedRuleset, classify.CurrentRuleset)
	}

	if observedRuleset != classify.CurrentRuleset {
		t.Errorf(
			"scan.domain_observed ruleset = %q, want %q (observations must carry evidence provenance)",
			observedRuleset,
			classify.CurrentRuleset,
		)
	}

	reader, err := scanstore.Boot(ctx, dsn)
	if err != nil {
		t.Fatalf("boot reader: %v", err)
	}

	defer func() { _ = reader.Close() }()

	replay, err := scanstore.ReplayJournal(ctx, reader, runID.Get())
	if err != nil {
		t.Fatalf("replay: %v", err)
	}

	if len(replay.Runs) != 1 {
		t.Fatalf("runs = %d, want 1", len(replay.Runs))
	}

	summary := replay.Runs[0]
	if summary.Wave != "tranco1m-a" || summary.Ruleset != classify.CurrentRuleset {
		t.Errorf("run summary provenance = wave %q ruleset %q, want wave %q ruleset %q",
			summary.Wave, summary.Ruleset, "tranco1m-a", classify.CurrentRuleset)
	}
}

// TestPreProvenanceJournalStillReplays pins additive evolution: a journal
// written before the wave/ruleset fields existed (zero values after decode)
// replays without error — old binaries' data must never strand.
func TestPreProvenanceJournalStillReplays(t *testing.T) {
	t.Parallel()

	payloadJSON := `{"run_id":"r1","started_at":"2026-11-10T00:00:00Z","input_domains":3,"skipped":0,"workers":4}`

	var legacy scanstore.ScanStartedPayload
	if err := json.Unmarshal([]byte(payloadJSON), &legacy); err != nil {
		t.Fatalf("decode legacy scan.started payload: %v", err)
	}

	if legacy.RunID != "r1" || legacy.Wave != "" || legacy.Ruleset != "" {
		t.Errorf("legacy payload decoded as %+v (wave/ruleset must default to zero, not error)", legacy)
	}
}
