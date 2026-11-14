package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	v5 "github.com/larsartmann/cmdguard/v5/pkg/cmdguard/v5"
)

// TestEnrichAppendsVerdictColumns pins the CSV enrichment mode (census T18):
// the input CSV streams through the event store's read model and comes back
// with every original column and row preserved, plus the normalized domain
// and derived verdicts. Unparseable cells and never-scanned domains keep
// their rows with explanatory error values — enrichment never drops rows.
func TestEnrichAppendsVerdictColumns(t *testing.T) {
	t.Parallel()

	store := filepath.Join(t.TempDir(), "scans.db")

	_, _, err := run(t, workspaceDomain(), "", "scan", "--no-ranges", "--event-store", store,
		"--format", "jsonl", "acme.com")
	if err != nil {
		t.Fatalf("seed scan failed: %v", err)
	}

	csvIn := "company,domain,tier\nAcme,https://acme.com/pricing,A\nBeta,not a domain,B\nGamma,missing.example,C\n"

	stdout, stderr, err := runRes(t, &fakeRes{}, csvIn,
		"enrich", "--event-store", store, "--input", "-")
	if err != nil {
		t.Fatalf("enrich failed: %v", err)
	}

	lines := strings.Split(strings.TrimRight(stdout, "\n"), "\n")
	if len(lines) != 4 {
		t.Fatalf("want header + 3 rows, got %d:\n%s", len(lines), stdout)
	}

	if lines[0] != "company,domain,tier,gdd_domain,workspace,gcp,error" {
		t.Errorf("header must append exactly the enrich columns, got: %s", lines[0])
	}

	if lines[1] != "Acme,https://acme.com/pricing,A,acme.com,certain,no," {
		t.Errorf("scanned row must carry derived verdicts, got: %s", lines[1])
	}

	if !strings.Contains(lines[2], "unparseable domain") || !strings.Contains(lines[2], "Beta") {
		t.Errorf("unparseable row must survive with an error value, got: %s", lines[2])
	}

	if !strings.Contains(lines[3], "not scanned") || !strings.Contains(lines[3], "Gamma") {
		t.Errorf("never-scanned row must survive with an error value, got: %s", lines[3])
	}

	if !strings.Contains(stderr, "3 rows enriched: 1 with recorded observations, 2 without") {
		t.Errorf("stderr must account the join, got:\n%s", stderr)
	}
}

// TestEnrichRejectsMissingStoreAndHeader pins the usage contract: no store
// or no recognizable domain column is exit 2, before any DNS or store work.
func TestEnrichRejectsMissingStoreAndHeader(t *testing.T) {
	t.Parallel()

	_, _, err := run(t, &fakeRes{}, "domain\nx.example\n", "enrich", "--input", "-")
	if code := v5.ExitCode(err); code != 2 {
		t.Errorf("enrich without --event-store must exit 2, got %d (%v)", code, err)
	}

	store := filepath.Join(t.TempDir(), "scans.db")

	if _, _, err2 := run(t, &fakeRes{}, "", "scan", "--no-ranges", "--event-store", store,
		"--format", "jsonl", "a.example"); err2 != nil {
		t.Fatalf("seed scan failed: %v", err2)
	}

	_, _, err = run(t, &fakeRes{}, "name,value\nfoo,bar\n", "enrich", "--event-store", store, "--input", "-")
	if code := v5.ExitCode(err); code != 2 {
		t.Errorf("enrich without a domain column must exit 2, got %d (%v)", code, err)
	}
}

// TestEnrichInputFileAndOpenError covers the file path of --input (the
// stdin '-' path is covered above): a real CSV file enriches like stdin,
// and a missing file fails with the path named instead of a usage error.
func TestEnrichInputFileAndOpenError(t *testing.T) {
	t.Parallel()

	store := filepath.Join(t.TempDir(), "scans.db")

	if _, _, err := run(t, &fakeRes{}, "", "scan", "--no-ranges", "--event-store", store,
		"--format", "jsonl", "acme.com"); err != nil {
		t.Fatalf("seed scan failed: %v", err)
	}

	csvPath := filepath.Join(t.TempDir(), "leads.csv")
	if err := os.WriteFile(csvPath, []byte("company,domain\nAcme,acme.com\n"), 0o600); err != nil {
		t.Fatalf("write input CSV: %v", err)
	}

	stdout, _, err := run(t, &fakeRes{}, "", "enrich", "--event-store", store, "--input", csvPath)
	if err != nil {
		t.Fatalf("enrich from file failed: %v", err)
	}

	if !strings.Contains(stdout, "company,domain,gdd_domain,workspace,gcp,error") {
		t.Errorf("enriched header missing appended columns, got: %q", stdout)
	}

	_, _, err = run(t, &fakeRes{}, "", "enrich", "--event-store", store,
		"--input", filepath.Join(t.TempDir(), "does-not-exist.csv"))
	if err == nil || !strings.Contains(err.Error(), "open --input") {
		t.Errorf("missing --input file must fail with the path named, got: %v", err)
	}
}

// TestEnrichShortRowSurvives covers the short-row branch: a CSV record
// with fewer cells than the header keeps its row with an explanatory
// error cell instead of failing the run.
func TestEnrichShortRowSurvives(t *testing.T) {
	t.Parallel()

	store := filepath.Join(t.TempDir(), "scans.db")

	if _, _, err := run(t, &fakeRes{}, "", "scan", "--no-ranges", "--event-store", store,
		"--format", "jsonl", "acme.com"); err != nil {
		t.Fatalf("seed scan failed: %v", err)
	}

	stdout, _, err := run(t, &fakeRes{}, "company,domain\nAcme\n", "enrich",
		"--event-store", store, "--input", "-")
	if err != nil {
		t.Fatalf("enrich with a short row must not fail: %v", err)
	}

	if !strings.Contains(stdout, "short row (no domain column)") {
		t.Errorf("short row must carry the explanatory cell, got: %q", stdout)
	}
}
