package scanstore

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/larsartmann/google-domain-detector/internal/classify"
)

func TestTmpFoldsVisibleWithoutHost(t *testing.T) {
	ctx := context.Background()
	store := filepath.Join(t.TempDir(), "s.db")

	sys, err := Open(ctx, store)
	if err != nil {
		t.Fatalf("open: %v", err)
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

	boot, err := Boot(ctx, store)
	if err != nil {
		t.Fatalf("boot: %v", err)
	}

	defer func() { _ = boot.Close() }()

	runs, err := ListRuns(ctx, boot)
	if err != nil {
		t.Fatalf("list: %v", err)
	}

	t.Logf("runs visible WITHOUT host started: %d", len(runs))
}
