package cli

import (
	"bytes"
	"context"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/larsartmann/google-domain-detector/internal/scanstore"
)

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

// TestScanCancelMidRunDrainsEventStore pins the G4 formality: Ctrl-C
// mid-scan still records every completed observation, CLOSES the run
// (CompleteScan survives the canceled context), and the partial run
// replays — the journal is a valid checkpoint to --resume from.
func TestScanCancelMidRunDrainsEventStore(t *testing.T) {
	t.Parallel()

	store := filepath.Join(t.TempDir(), "scans.db")

	var stderr bytes.Buffer

	stdout := &snapshotWriter{}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	d, err := BuildCLI(
		stdout,
		&stderr,
		"test",
		"",
		&hangRes{fakeRes: &fakeRes{}, cancel: cancel, wait: stdout},
		strings.NewReader(""),
		false,
		false,
	)
	if err != nil {
		t.Fatalf("BuildCLI failed: %v", err)
	}

	if err := d.ExecuteWithArgs(
		ctx,
		[]string{"scan", "--no-ranges", "--format", "jsonl", "--event-store", store, "fast1.com", "slow.com"},
	); err != nil {
		t.Fatalf("canceled scan must still finish cleanly: %v", err)
	}

	if !strings.Contains(stderr.String(), "run ") || !strings.Contains(stderr.String(), "recorded") {
		t.Fatalf("event store hint missing from stderr:\n%s", stderr.String())
	}

	reader, err := scanstore.Boot(context.Background(), store)
	if err != nil {
		t.Fatalf("boot store: %v", err)
	}

	defer func() { _ = reader.Close() }()

	replay, err := scanstore.ReplayJournal(context.Background(), reader, "")
	if err != nil {
		t.Fatalf("replay partial run: %v", err)
	}

	if len(replay.Results) != 2 {
		t.Fatalf(
			"partial run must journal both domains (completed + canceled), got %d: %+v",
			len(replay.Results),
			replay.Results,
		)
	}

	for _, run := range replay.Runs {
		if !run.Completed {
			t.Errorf("run must be completed despite cancellation: %+v", run)
		}

		if run.Counts.Scanned != 2 || run.Counts.Errors != 1 {
			t.Errorf("run counts = %+v, want 2 scanned / 1 error (canceled slow)", run.Counts)
		}
	}
}
