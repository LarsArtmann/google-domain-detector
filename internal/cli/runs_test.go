package cli

import (
	"context"
	"encoding/json/v2"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/larsartmann/google-domain-detector/internal/classify"
	"github.com/larsartmann/google-domain-detector/internal/scanstore"
)

// runsTestStore records two runs and closes the store: run1 completed with
// a tally and a wave name; run2 resumed from run1 and never completed.
// Returns the store path and both FULL run IDs.
func runsTestStore(t *testing.T) (string, string, string) {
	t.Helper()

	ctx := context.Background()
	store := filepath.Join(t.TempDir(), "scans.db")

	sys, err := scanstore.Open(ctx, store)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}

	rec := scanstore.NewRecorder(sys)

	first := scanstore.NewRunID()
	if err := rec.StartScan(ctx, first, "", "wave-test", 2, 0, 4); err != nil {
		t.Fatalf("start first run: %v", err)
	}

	if err := rec.Observe(ctx, first, classify.Result{Domain: "a.example"}); err != nil {
		t.Fatalf("observe a.example: %v", err)
	}

	if err := rec.CompleteScan(ctx, first, time.Second, classify.Counts{
		Scanned:          1,
		WorkspaceCertain: 1,
	}); err != nil {
		t.Fatalf("complete first run: %v", err)
	}

	second := scanstore.NewRunID()
	if err := rec.StartScan(ctx, second, first.Get(), "", 5, 1, 4); err != nil {
		t.Fatalf("start resumed run: %v", err)
	}

	if err := sys.GracefulClose(ctx); err != nil {
		t.Fatalf("close store: %v", err)
	}

	return store, first.Get(), second.Get()
}

// TestRunsJSONLCarriesFullRunIDs pins the reason --format jsonl exists: the
// 12-char table truncation once forced raw SQL to recover run IDs for
// --run/--resume/audit selection. Every machine format must carry the FULL
// IDs plus the raw run fields (wave, resumed_from, counts).
func TestRunsJSONLCarriesFullRunIDs(t *testing.T) {
	t.Parallel()

	store, first, second := runsTestStore(t)

	stdout, _, err := run(t, &fakeRes{}, "", "runs", "--event-store", store, "--format", "jsonl")
	if err != nil {
		t.Fatalf("runs jsonl failed: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 jsonl lines, got %d: %q", len(lines), stdout)
	}

	byID := map[string]map[string]any{}

	for _, line := range lines {
		var row map[string]any

		if err := json.Unmarshal([]byte(line), &row); err != nil {
			t.Fatalf("decode line %q: %v", line, err)
		}

		id, _ := row["run_id"].(string)
		byID[id] = row
	}

	if len(byID) != 2 {
		t.Fatalf("expected 2 distinct full run IDs, got %d: %v", len(byID), byID)
	}

	for _, id := range []string{first, second} {
		if _, ok := byID[id]; !ok {
			t.Fatalf("full run ID %s missing from jsonl output: %v", id, byID)
		}

		if strings.Contains(id, "…") || len(id) <= 12 {
			t.Fatalf("test invariant broken: run ID %q is not a full ID", id)
		}
	}

	completed := byID[first]
	if got, _ := completed["wave"].(string); got != "wave-test" {
		t.Errorf("completed run should carry its wave name, got %q", got)
	}

	if got, _ := completed["completed"].(bool); !got {
		t.Errorf("completed run should report completed=true")
	}

	counts, _ := completed["counts"].(map[string]any)
	if got, _ := counts["workspace_certain"].(float64); got != 1 {
		t.Errorf("counts.workspace_certain should round-trip, got %v", got)
	}

	resumed := byID[second]
	if got, _ := resumed["resumed_from"].(string); got != first {
		t.Errorf("resumed run should name its resume source, got %q", got)
	}

	if got, _ := resumed["completed"].(bool); got {
		t.Errorf("interrupted run should report completed=false")
	}
}

// TestRunsJSONArrayFormat pins the json format: the same full-ID DTOs under
// an array wrapper.
func TestRunsJSONArrayFormat(t *testing.T) {
	t.Parallel()

	store, first, second := runsTestStore(t)

	stdout, _, err := run(t, &fakeRes{}, "", "runs", "--event-store", store, "--format", "json")
	if err != nil {
		t.Fatalf("runs json failed: %v", err)
	}

	var rows []map[string]any

	if err := json.Unmarshal([]byte(stdout), &rows); err != nil {
		t.Fatalf("decode json array: %v", err)
	}

	if len(rows) != 2 {
		t.Fatalf("expected 2 runs, got %d: %q", len(rows), stdout)
	}

	seen := map[string]bool{}

	for _, row := range rows {
		id, _ := row["run_id"].(string)
		seen[id] = true
	}

	if !seen[first] || !seen[second] {
		t.Errorf("json array must carry both FULL run IDs, got %v", seen)
	}
}

// TestRunsDataFormatsKeepFullIDsAndStayPure pins the data-format law for
// csv: full run IDs, and no human decorations (truncated IDs, footer
// counts, table title) leaking into the data.
func TestRunsDataFormatsKeepFullIDsAndStayPure(t *testing.T) {
	t.Parallel()

	store, first, _ := runsTestStore(t)

	stdout, _, err := run(t, &fakeRes{}, "", "runs", "--event-store", store, "--format", "csv")
	if err != nil {
		t.Fatalf("runs csv failed: %v", err)
	}

	if !strings.Contains(stdout, first) {
		t.Errorf("csv must carry the FULL run ID %s, got: %q", first, stdout)
	}

	for _, decoration := range []string{"…", "run(s)", "Scan Runs"} {
		if strings.Contains(stdout, decoration) {
			t.Errorf("csv must stay pure data: found %q in %q", decoration, stdout)
		}
	}
}

// TestRunsTableStaysReadable pins the human side of the split: the default
// table may truncate IDs and carry the run-count footer.
func TestRunsTableStaysReadable(t *testing.T) {
	t.Parallel()

	store, _, _ := runsTestStore(t)

	stdout, _, err := run(t, &fakeRes{}, "", "runs", "--event-store", store)
	if err != nil {
		t.Fatalf("runs table failed: %v", err)
	}

	if !strings.Contains(stdout, "…") {
		t.Errorf("table is allowed to truncate run IDs and should, got: %q", stdout)
	}

	if !strings.Contains(stdout, "2 run(s)") {
		t.Errorf("table footer should count the runs, got: %q", stdout)
	}
}
