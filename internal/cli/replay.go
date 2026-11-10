package cli

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/larsartmann/go-output"

	"github.com/larsartmann/google-domain-detector/internal/classify"
	"github.com/larsartmann/google-domain-detector/internal/render"
	"github.com/larsartmann/google-domain-detector/internal/scanstore"
)

// ReplayFlags is the replay command's flag set.
type ReplayFlags struct {
	EventStore string `env:"EVENT_STORE" flag:"event-store" help:"SQLite event store written by scan --event-store"`
	Run        string `                  flag:"run"         help:"Rebuild this run ID (default: the most recent run)"`
	Format     string `                  flag:"format"      help:"Output format"                                      default:"table" values:"table,json,csv,tsv,markdown,yaml,toml,jsonl"`
	Only       string `                  flag:"only"        help:"Report only matching domains"                       default:"all"   values:"all,workspace,gcp,google"`
}

// runReplay rebuilds a scan report from the event journal alone: no DNS.
func runReplay(ctx context.Context, f ReplayFlags, stdout, stderr io.Writer) error {
	if f.EventStore == "" {
		return usageError(errors.New("--event-store is required (or set GDD_EVENT_STORE)"))
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

	// jsonl streams straight out of the journal walk — a whole-TLD store
	// never materializes; every other format rebuilds fully first.
	if format == output.FormatJSONL {
		stream := render.NewJSONLStream(stdout)

		summary, err := scanstore.StreamObservations(ctx, sys, f.Run, func(r classify.Result) error {
			if render.Matches(r, filter) {
				stream.Write(r)
			}

			return nil
		})
		if err != nil {
			if errors.Is(err, scanstore.ErrNoRuns) {
				return fmt.Errorf("no scan runs found in %s", f.EventStore)
			}

			return err
		}

		fmt.Fprintf(stderr, "run %s: %d domains, workspace certain %d / likely %d, gcp %d, errors %d%s\n",
			summary.RunID, summary.Counts.Scanned,
			summary.Counts.WorkspaceCertain, summary.Counts.WorkspaceLikely,
			summary.Counts.GCPCertain, summary.Counts.Errors, errorKindsSuffix(summary.Counts))

		return stream.Err()
	}

	replay, err := scanstore.ReplayJournal(ctx, sys, f.Run)
	if err != nil {
		if errors.Is(err, scanstore.ErrNoRuns) {
			return fmt.Errorf("no scan runs found in %s", f.EventStore)
		}

		return err
	}

	var summary *scanstore.RunSummary

	for i := range replay.Runs {
		if replay.Runs[i].RunID == replay.RunID {
			summary = &replay.Runs[i]

			break
		}
	}

	if summary != nil {
		fmt.Fprintf(stderr, "run %s: %d domains, workspace certain %d / likely %d, gcp %d, errors %d%s\n",
			summary.RunID, summary.Counts.Scanned,
			summary.Counts.WorkspaceCertain, summary.Counts.WorkspaceLikely,
			summary.Counts.GCPCertain, summary.Counts.Errors, errorKindsSuffix(summary.Counts))
	}

	return renderResults(stdout, replay.Results, filter, format)
}
