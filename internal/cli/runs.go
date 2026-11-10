package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
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
		return usageError(errors.New("--event-store is required (or set GDD_EVENT_STORE)"))
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
		return fmt.Errorf("no scan runs found in %s", f.EventStore)
	}

	table := output.NewTable([]string{"Run", "Started", "Status", "Domains", "Workspace", "GCP", "Errors", "Wave"})

	for _, run := range runs {
		started := "unknown"
		if !run.StartedAt.IsZero() {
			started = run.StartedAt.Local().Format(time.DateTime)
		}

		status := "started"
		if run.Completed {
			status = "completed"
		}

		if run.ResumedFrom != "" {
			status += " (resumed)"
		}

		table.AddRow([]string{
			shortRunID(run.RunID),
			started,
			status,
			fmt.Sprintf("%d", run.Counts.Scanned),
			fmt.Sprintf("%d/%d", run.Counts.WorkspaceCertain, run.Counts.WorkspaceLikely),
			fmt.Sprintf("%d", run.Counts.GCPCertain),
			fmt.Sprintf("%d", run.Counts.Errors),
			run.Wave,
		})
	}

	fmt.Fprintf(stderr, "%d run(s); rebuild one with: %s replay --event-store %s --run <id>\n",
		len(runs), Name, f.EventStore)

	return output.RenderTable(table, output.FormatTable, output.RenderOptions{Writer: stdout})
}

// shortRunID keeps the table readable: a run ID's first label pair is unique
// enough for a list, the full ID stays one --run flag away.
func shortRunID(runID string) string {
	if len(runID) > 12 {
		return runID[:12] + "…"
	}

	return runID
}
