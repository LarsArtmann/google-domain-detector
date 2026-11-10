package cli

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/larsartmann/go-output"

	"github.com/larsartmann/google-domain-detector/internal/render"
	"github.com/larsartmann/google-domain-detector/internal/scanstore"
)

// ReplayFlags is the replay command's flag set.
type ReplayFlags struct {
	EventStore string `flag:"event-store" help:"SQLite event store written by scan --event-store" env:"GDD_EVENT_STORE"`
	Run        string `flag:"run"          help:"Rebuild this run ID (default: the most recent run)"`
	Format     string `flag:"format"       help:"Output format"  default:"table" values:"table,json,csv,tsv,markdown,yaml,toml,jsonl"`
	Only       string `flag:"only"         help:"Report only matching domains" default:"all" values:"all,workspace,gcp,google"`
}

// runReplay rebuilds a scan report from the event journal alone: no DNS.
func runReplay(ctx context.Context, f ReplayFlags, stdout, stderr io.Writer) error {
	if f.EventStore == "" {
		return errors.New("--event-store is required (or set GDD_EVENT_STORE)")
	}

	filter, err := render.ParseFilter(f.Only)
	if err != nil {
		return err
	}

	format, err := output.ParseFormat(f.Format)
	if err != nil {
		return err
	}

	sys, err := scanstore.Boot(ctx, f.EventStore)
	if err != nil {
		return fmt.Errorf("open event store %s: %w", f.EventStore, err)
	}

	defer func() { _ = sys.Close() }()

	replay, err := scanstore.ReplayJournal(ctx, sys, f.Run)
	if err != nil {
		return err
	}

	if len(replay.Runs) == 0 {
		return fmt.Errorf("no scan runs found in %s", f.EventStore)
	}

	var summary *scanstore.RunSummary

	for i := range replay.Runs {
		if replay.Runs[i].RunID == replay.RunID {
			summary = &replay.Runs[i]

			break
		}
	}

	if summary != nil {
		fmt.Fprintf(stderr, "run %s: %d domains, workspace certain %d / likely %d, gcp %d, errors %d\n",
			summary.RunID, summary.Counts.Scanned,
			summary.Counts.WorkspaceCertain, summary.Counts.WorkspaceLikely,
			summary.Counts.GCPCertain, summary.Counts.Errors)
	}

	return renderResults(stdout, replay.Results, filter, format)
}
