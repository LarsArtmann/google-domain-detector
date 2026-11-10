package cli

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"time"

	"github.com/larsartmann/go-output"

	"github.com/larsartmann/google-domain-detector/internal/scanstore"
)

// RunsFlags is the runs command's flag set.
type RunsFlags struct {
	EventStore string `env:"EVENT_STORE" flag:"event-store" help:"SQLite event store written by scan --event-store"`
}

// runRuns lists every recorded scan run from the read model, newest first,
// with its status and verdict tally. No DNS, no journal walk.
func runRuns(ctx context.Context, f RunsFlags, stdout, stderr io.Writer) error {
	if f.EventStore == "" {
		return usageError(errEventStoreRequired)
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

	table := output.NewTable([]string{"Run", "Started", "Status", "Domains", "Workspace", "GCP", "Errors", "Wave"})

	for _, run := range runs {
		started := "unknown"
		if !run.StartedAt.IsZero() {
			// The operator's wall clock, not a fixed zone: the ledger is
			// read by humans sitting at this machine.
			//nolint:gosmopolitan // deliberate local-time rendering
			started = run.StartedAt.Local().Format(time.DateTime)
		}

		status := "started"
		if run.Completed {
			status = "completed"
		}

		if run.ResumedFrom != "" {
			status += " (resumed)"
		}

		if err := table.AddRowChecked([]string{
			shortRunID(run.RunID),
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

	table.SetFooter([]string{fmt.Sprintf("%d run(s)", len(runs)), "", "", "", "", "", "", ""})

	_, _ = fmt.Fprintf(stderr, "%d run(s); rebuild one with: %s replay --event-store %s --run <id>\n",
		len(runs), Name, f.EventStore)

	renderOpts := output.RenderOptions{Writer: stdout, Title: "Scan Runs"}

	return output.RenderTable(table, output.FormatTable, renderOpts)
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
