package cli

import (
	"bytes"
	"context"
	"errors"
	"io"
	"path/filepath"
	"strings"
	"testing"

	"github.com/larsartmann/google-domain-detector/internal/scanstore"
)

// TestEnrichMalformedCSVFailsCleanly covers the row-read error branch: a
// bare-quote line must abort with the file line named, not silently drop.
func TestEnrichMalformedCSVFailsCleanly(t *testing.T) {
	t.Parallel()

	store := filepath.Join(t.TempDir(), "scans.db")

	var out, errOut bytes.Buffer

	err := runEnrich(context.Background(), EnrichFlags{EventStore: store, Input: "-"},
		strings.NewReader("domain\nacme.com\nbad \"quote\n"), &out, &errOut)
	if err == nil || !strings.Contains(err.Error(), "read CSV line") {
		t.Errorf("malformed CSV must fail naming the line, got: %v", err)
	}
}

// failingWriter covers the CSV write-error branches of enrichRows.
type failingWriter struct{ after int }

func (w *failingWriter) Write(p []byte) (int, error) {
	if w.after <= 0 {
		return 0, errors.New("disk full")
	}

	w.after--

	return len(p), nil
}

func TestEnrichWriteErrorAborts(t *testing.T) {
	t.Parallel()

	store := filepath.Join(t.TempDir(), "scans.db")

	err := runEnrich(context.Background(), EnrichFlags{EventStore: store, Input: "-"},
		strings.NewReader("domain\nacme.com\nother.example\n"), &failingWriter{}, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "write CSV") {
		t.Errorf("writer failure must abort with the line named, got: %v", err)
	}
}

// TestEnrichRowStoreReadFailed covers the read-model failure branch: the
// row survives with a store-read error cell instead of aborting.
// TestEnrichRowStoreReadFailed covers the read-model failure branch: the
// row survives with a store-read error cell instead of aborting.
func TestEnrichRowStoreReadFailed(t *testing.T) {
	t.Parallel()

	store := filepath.Join(t.TempDir(), "scans.db")

	sys, err := scanstore.Open(context.Background(), store)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}

	defer func() { _ = sys.Close() }()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	verdict := enrichRow(ctx, sys, []string{"acme.com"}, 0)
	if verdict.scanned || !strings.Contains(verdict.cells[3], "store read failed") {
		t.Errorf("canceled read must surface the store-read-failed cell, got: %+v", verdict)
	}
}
