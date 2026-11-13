package cli

import (
	"context"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"fmt"
	"io"
	"strconv"
	"time"

	"github.com/larsartmann/go-output"

	"github.com/larsartmann/google-domain-detector/internal/classify"
	"github.com/larsartmann/google-domain-detector/internal/scanstore"
)

//nolint:golines // fleet golines pads/reorders tags against the project tagalign policy; tags cannot wrap (see .golangci.yml)
type RunsFlags struct {
	EventStore string `env:"EVENT_STORE" flag:"event-store" help:"SQLite event store written by scan --event-store"`
	Format     string `default:"table" flag:"format" help:"Output format" values:"table,json,csv,tsv,markdown,yaml,toml,jsonl"`
	Color      string `default:"auto" flag:"color" help:"Terminal colors for table output" values:"auto,always,never"`
}

// runRuns lists every recorded scan run from the read model, newest first,
// with its status and verdict tally. No DNS, no journal walk. Human formats
// (table, markdown) shorten run IDs for readability; every data format
// (json, jsonl, csv, tsv, yaml, toml) carries the FULL IDs — a truncated
// ID is useless for --run/--resume/audit selection.
func runRuns(ctx context.Context, f RunsFlags, stdout, stderr io.Writer) error {
	if f.EventStore == "" {
		return usageError(errEventStoreRequired)
	}

	format, err := output.ParseFormat(f.Format)
	if err != nil {
		return usageError(err)
	}

	colorMode, err := parseColorMode(f.Color)
	if err != nil {
		return err
	}

	sys, err := scanstore.Open(ctx, f.EventStore)
	if err != nil {
		return fmt.Errorf("open event store %s: %w", f.EventStore, err)
	}

	defer func() { _ = sys.Close() }()

	runs, err := scanstore.ListRuns(ctx, sys)
	if err != nil {
		return err
	}

	if len(runs) == 0 {
		return fmt.Errorf("%s: %w", f.EventStore, errNoRuns)
	}

	_, _ = fmt.Fprintf(stderr, "%d run(s); rebuild one with: %s replay --event-store %s --run <id>\n",
		len(runs), Name, f.EventStore)

	if format == output.FormatJSON || format == output.FormatJSONL {
		return renderRunsJSON(stdout, runs, format)
	}

	shortIDs := format == output.FormatTable || format == output.FormatMarkdown

	table := output.NewTable([]string{"Run", "Started", "Status", "Domains", "Workspace", "GCP", "Errors", "Wave"})
	for _, run := range runs {
		started := "unknown"
		if !run.StartedAt.IsZero() {
			// The operator's wall clock, not a fixed zone: the ledger is
			// read by humans sitting at this machine.
			//nolint:gosmopolitan // deliberate local-time rendering
			started = run.StartedAt.Local().Format(time.DateTime)
		}

		id := run.RunID
		if shortIDs {
			id = shortRunID(run.RunID)
		}

		status := "started"
		if run.Completed {
			status = "completed"
		}

		if run.ResumedFrom != "" {
			status += " (resumed)"
		}

		if err := table.AddRowChecked([]string{
			id,
			started,
			status,
			strconv.Itoa(run.Counts.Scanned),
			strconv.Itoa(run.Counts.WorkspaceCertain) + "/" + strconv.Itoa(run.Counts.WorkspaceLikely),
			strconv.Itoa(run.Counts.GCPCertain),
			strconv.Itoa(run.Counts.Errors),
			run.Wave,
		}); err != nil {
			return fmt.Errorf("add row for run %s: %w", run.RunID, err)
		}
	}

	// Footer is a human decoration: data formats (csv/tsv/yaml/toml) must
	// stay pure rows — go-output renders whatever is set, so gate it here.
	if shortIDs {
		table.SetFooter([]string{fmt.Sprintf("%d run(s)", len(runs)), "", "", "", "", "", "", ""})
	}

	renderOpts := output.RenderOptions{Writer: stdout, ColorMode: colorMode, Title: "Scan Runs"}

	return output.RenderTable(table, format, renderOpts)
}

// runJSON is the machine contract of one run: raw fields only, FULL run ID,
// timestamps in RFC3339 via time.Time. Status is derived by the consumer
// from completed + resumed_from — the DTO never duplicates derivations.
type runJSON struct {
	RunID       string          `json:"run_id"`
	StartedAt   time.Time       `json:"started_at"`
	Completed   bool            `json:"completed"`
	ResumedFrom string          `json:"resumed_from,omitempty"`
	Wave        string          `json:"wave,omitempty"`
	Counts      classify.Counts `json:"counts"`
}

func runJSONFrom(r scanstore.RunView) runJSON {
	return runJSON{
		RunID:       r.RunID,
		StartedAt:   r.StartedAt,
		Completed:   r.Completed,
		ResumedFrom: r.ResumedFrom,
		Wave:        r.Wave,
		Counts:      r.Counts,
	}
}

func renderRunsJSON(w io.Writer, runs []scanstore.RunView, format output.Format) error {
	dtOs := make([]runJSON, 0, len(runs))
	for _, r := range runs {
		dtOs = append(dtOs, runJSONFrom(r))
	}

	if format == output.FormatJSON {
		enc := jsontext.NewEncoder(w)

		return json.MarshalEncode(enc, dtOs)
	}

	for _, dto := range dtOs {
		line, err := json.Marshal(dto)
		if err != nil {
			return fmt.Errorf("encode run %s: %w", dto.RunID, err)
		}

		if _, err := fmt.Fprintf(w, "%s\n", line); err != nil {
			return fmt.Errorf("write run line: %w", err)
		}
	}

	return nil
}

// shortRunID keeps the table readable: a run ID's first label pair is unique
// enough for a list, the full ID stays one --run flag away.
func shortRunID(runID string) string {
	const shortRunIDLength = 12

	if len(runID) > shortRunIDLength {
		return runID[:shortRunIDLength] + "…"
	}

	return runID
}
