package cli

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"

	cqrs "github.com/larsartmann/go-cqrs-lite/system/v4"

	"github.com/larsartmann/google-domain-detector/internal/classify"
	"github.com/larsartmann/google-domain-detector/internal/domain"
	"github.com/larsartmann/google-domain-detector/internal/input"
	"github.com/larsartmann/google-domain-detector/internal/scanstore"
)

// EnrichFlags is the enrich command's flag set.
//

type EnrichFlags struct {
	EventStore string `env:"EVENT_STORE" flag:"event-store" help:"SQLite event store written by scan --event-store"`
	Input      string `                  flag:"input"       help:"CSV with a domain/website/url column ('-' reads stdin)" short:"i"`
}

var (
	errEnrichNeedsStore = errors.New(
		"enrich needs --event-store: the CSV is joined against recorded observations, no DNS runs",
	)
	errEnrichNeedsCSV = errors.New(
		"enrich needs --input FILE: a CSV whose header carries a domain/website/url column",
	)
	errEnrichNoHeader = errors.New(
		"enrich input has no recognizable header: add a domain, website, or url column header (enrichment preserves the original columns, so it must know where the domains live)",
	)
)

// enrichColumns appended to every row: the normalized domain the verdicts
// belong to (inputs may carry URLs or www forms) plus the verdicts.
var enrichColumns = []string{"gdd_domain", "workspace", "gcp", "error"}

// runEnrich streams the input CSV through the event store's read model and
// writes it back with appended verdict columns — every original column and
// row preserved verbatim (input parsing never drops rows). Verdicts come
// from the domain's LATEST observation across all runs; unparseable domain
// cells and never-scanned domains keep their row with an explanatory error
// value instead of a verdict.
func runEnrich(ctx context.Context, f EnrichFlags, stdin io.Reader, stdout, stderr io.Writer) error {
	if f.EventStore == "" {
		return usageError(errEnrichNeedsStore)
	}

	if f.Input == "" {
		return usageError(errEnrichNeedsCSV)
	}

	source := stdin

	if f.Input != "-" {
		file, err := os.Open(f.Input)
		if err != nil {
			return fmt.Errorf("open --input %s: %w", f.Input, err)
		}

		defer func() { _ = file.Close() }()

		source = file
	}

	reader := csv.NewReader(source)
	reader.FieldsPerRecord = -1

	header, err := reader.Read()
	if err != nil {
		return fmt.Errorf("read CSV header: %w", err)
	}

	col, ok := input.HeaderColumn(header)
	if !ok {
		return usageError(fmt.Errorf("%w (saw: %s)", errEnrichNoHeader, strings.Join(header, ", ")))
	}

	sys, err := scanstore.Open(ctx, f.EventStore)
	if err != nil {
		return fmt.Errorf("open event store %s: %w", f.EventStore, err)
	}

	defer func() { _ = sys.Close() }()

	writer := csv.NewWriter(stdout)
	defer writer.Flush()

	if err := writer.Write(append(slices.Clone(header), enrichColumns...)); err != nil {
		return fmt.Errorf("write header: %w", err)
	}

	rows, enriched := 0, 0

	for {
		record, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}

		if err != nil {
			return fmt.Errorf("read CSV row %d: %w", rows+2, err) // +2: 1-based row, past the header
		}

		rows++

		verdict := enrichRow(ctx, sys, record, col)
		if verdict.scanned {
			enriched++
		}

		if err := writer.Write(append(slices.Clone(record), verdict.cells...)); err != nil {
			return fmt.Errorf("write CSV row %d: %w", rows+1, err)
		}
	}

	if err := writer.Error(); err != nil {
		return fmt.Errorf("write CSV: %w", err)
	}

	_, _ = fmt.Fprintf(
		stderr,
		"%d rows enriched: %d with recorded observations, %d without (scan the missing domains first, then re-run)\n",
		rows,
		enriched,
		rows-enriched,
	)

	return nil
}

// enrichVerdict is one row's appended cells; scanned reports whether the
// domain had a recorded observation to derive verdicts from.
type enrichVerdict struct {
	cells   []string
	scanned bool
}

// enrichRow derives the appended cells for one CSV record. It never fails
// the run for a bad cell — the row survives with an explanatory error.
func enrichRow(ctx context.Context, sys *cqrs.System, record []string, col int) enrichVerdict {
	if col >= len(record) {
		return enrichVerdict{cells: []string{"", "", "", "short row (no domain column)"}, scanned: false}
	}

	dom, err := domain.Parse(record[col])
	if err != nil {
		return enrichVerdict{cells: []string{"", "", "", "unparseable domain"}, scanned: false}
	}

	view, found, err := scanstore.LatestObservation(ctx, sys, dom.String())
	if err != nil {
		return enrichVerdict{cells: []string{dom.String(), "", "", "store read failed: " + err.Error()}, scanned: false}
	}

	if !found {
		return enrichVerdict{cells: []string{dom.String(), "", "", "not scanned"}, scanned: false}
	}

	result := classify.Result{
		Domain:    view.Domain,
		Evidences: view.Evidence,
		Err:       view.Err,
		ErrKind:   view.ErrKind,
	}

	workspace := "no"
	if w := result.WorkspaceVerdict(); w.Detected {
		workspace = string(w.Confidence) // certain | likely
	}

	gcp := "no"
	if result.GCPVerdict().Detected {
		gcp = "yes"
	}

	errKind := ""
	if result.Err != "" {
		errKind = string(result.ErrKind)
	}

	return enrichVerdict{cells: []string{dom.String(), workspace, gcp, errKind}, scanned: true}
}
