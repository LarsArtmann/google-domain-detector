package scanstore_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/larsartmann/google-domain-detector/internal/classify"
	"github.com/larsartmann/google-domain-detector/internal/scanstore"
)

// TestJournalOneHundredThousandEvents is the G4 benchmark: 100k observations
// append and replay within a sane budget, and the store size stays linear.
func TestJournalOneHundredThousandEvents(t *testing.T) {
	t.Parallel()

	// ~8-10 min under -race locally, and it blew CI's default 10-min go test
	// budget on 2-core runners twice (600s panic, goroutines idle). Perf
	// soaks are opt-in, not functional gates; measured numbers: AGENTS.md G4.
	if os.Getenv("GDD_SOAK") == "" {
		t.Skip("set GDD_SOAK=1 to run the 100k-event journal soak (exceeds CI's default test budget)")
	}

	ctx := context.Background()
	dsn := filepath.Join(t.TempDir(), "scans.db")

	writer, err := scanstore.Open(ctx, dsn)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}

	recorder := scanstore.NewRecorder(writer)

	runID := scanstore.NewRunID()

	const total = 100_000

	appendStart := time.Now()

	if err := recorder.StartScan(ctx, runID, "", "", total, 0, 8); err != nil {
		t.Fatalf("start run: %v", err)
	}

	for i := range total {
		if err := recorder.Observe(ctx, runID, classify.Result{Domain: fmt.Sprintf("d%d.example", i)}); err != nil {
			t.Fatalf("observe %d: %v", i, err)
		}
	}

	if err := recorder.CompleteScan(ctx, runID, time.Minute, classify.Counts{Scanned: total}); err != nil {
		t.Fatalf("complete run: %v", err)
	}

	if err := writer.GracefulClose(ctx); err != nil {
		t.Fatalf("close writer: %v", err)
	}

	appendWall := time.Since(appendStart)

	reader, err := scanstore.Boot(ctx, dsn)
	if err != nil {
		t.Fatalf("boot reader: %v", err)
	}

	defer func() { _ = reader.Close() }()

	var replayed int

	replayStart := time.Now()

	if _, err := scanstore.StreamObservations(ctx, reader, "", func(classify.Result) error {
		replayed++

		return nil
	}); err != nil {
		t.Fatalf("stream replay: %v", err)
	}

	replayWall := time.Since(replayStart)

	size, err := os.Stat(dsn)
	if err != nil {
		t.Fatalf("stat store: %v", err)
	}

	var mem runtime.MemStats

	runtime.ReadMemStats(&mem)

	if replayed != total {
		t.Errorf("replayed %d of %d observations", replayed, total)
	}

	t.Logf("100k events: append %s (%.0f/s), replay %s (%.0f/s), db %d MiB, heap in use %d MiB",
		appendWall, float64(total)/appendWall.Seconds(),
		replayWall, float64(total)/replayWall.Seconds(),
		size.Size()/(1<<20), mem.HeapInuse/(1<<20))
}
