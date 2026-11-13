package scanstore

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/larsartmann/google-domain-detector/internal/classify"
)

func recordTwoRuns(t *testing.T, store string) {
	t.Helper()

	ctx := context.Background()

	sys, err := Boot(ctx, store)
	if err != nil {
		t.Fatalf("boot writer: %v", err)
	}

	rec := NewRecorder(sys)
	id := NewRunID()

	if err := rec.StartScan(ctx, id, "", "w", 1, 0, 1); err != nil {
		t.Fatalf("start: %v", err)
	}

	if err := rec.Observe(ctx, id, classify.Result{Domain: "a.example"}); err != nil {
		t.Fatalf("observe: %v", err)
	}

	if err := sys.GracefulClose(ctx); err != nil {
		t.Fatalf("close: %v", err)
	}
}

// Experiment A: record with the host NEVER started (Boot only). If folds
// are pure dispatch-synchronous, ListRuns on a Boot-only reopen sees rows.
func TestTmpRecordWithoutHostFoldsStillVisible(t *testing.T) {
	ctx := context.Background()
	store := filepath.Join(t.TempDir(), "s.db")

	recordTwoRuns(t, store)

	boot, err := Boot(ctx, store)
	if err != nil {
		t.Fatalf("boot reader: %v", err)
	}

	defer func() { _ = boot.Close() }()

	runs, err := ListRuns(ctx, boot)
	if err != nil {
		t.Fatalf("list: %v", err)
	}

	t.Logf("record-without-host: ListRuns saw %d run(s)", len(runs))

	if len(runs) != 1 {
		t.Errorf("folds should be dispatch-synchronous without any host, saw %d", len(runs))
	}
}

// Experiment B: the production shape (Open = Boot + Start) reopened in a
// tight loop — does an immediate ListRuns ever see ZERO runs?
func TestTmpReopenWithStartRaceLoop(t *testing.T) {
	ctx := context.Background()
	store := filepath.Join(t.TempDir(), "s.db")

	recordTwoRuns(t, store)

	for i := range 60 {
		sys, err := Open(ctx, store)
		if err != nil {
			t.Fatalf("open %d: %v", i, err)
		}

		runs, err := ListRuns(ctx, sys)
		if err != nil {
			_ = sys.Close()
			t.Fatalf("list %d: %v", i, err)
		}

		if len(runs) == 0 {
			_ = sys.Close()
			t.Fatalf("iteration %d: ListRuns saw ZERO runs right after Open — race reproduced", i)
		}

		if err := sys.Close(); err != nil {
			t.Fatalf("close %d: %v", i, err)
		}
	}
}
