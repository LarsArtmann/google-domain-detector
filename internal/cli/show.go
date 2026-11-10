package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/larsartmann/go-output"

	"github.com/larsartmann/google-domain-detector/internal/classify"
	"github.com/larsartmann/google-domain-detector/internal/domain"
	"github.com/larsartmann/google-domain-detector/internal/render"
	"github.com/larsartmann/google-domain-detector/internal/scanstore"
)

// ShowFlags is the show command's flag set.
type ShowFlags struct {
	EventStore string `env:"EVENT_STORE" flag:"event-store" help:"SQLite event store written by scan --event-store"`
	Format     string `                 flag:"format"      help:"Output format"             default:"table" values:"table,json,csv,tsv,markdown,yaml,toml,jsonl"`
}

var errShowNeedsOneDomain = errors.New("show needs exactly one domain: show example.com --event-store scans.db")

// runShow prints one domain's latest observation from the read model: the
// verdicts its evidence implies, how often the store has observed it, and
// which run saw it last. No DNS, no journal walk.
func runShow(ctx context.Context, f ShowFlags, args []string, stdout, stderr io.Writer) error {
	if f.EventStore == "" {
		return usageError(errors.New("--event-store is required (or set GDD_EVENT_STORE)"))
	}

	if len(args) != 1 {
		return usageError(errShowNeedsOneDomain)
	}

	d, err := domain.Parse(args[0])
	if err != nil {
		return usageError(fmt.Errorf("not a domain: %q: %w", args[0], err))
	}

	format, err := output.ParseFormat(f.Format)
	if err != nil {
		return usageError(err)
	}

	sys, err := scanstore.Open(ctx, f.EventStore)
	if err != nil {
		return fmt.Errorf("open event store %s: %w", f.EventStore, err)
	}

	defer func() { _ = sys.Close() }()

	view, found, err := scanstore.LatestObservation(ctx, sys, d.String())
	if err != nil {
		return err
	}

	if !found {
		return fmt.Errorf("no observations for %s in %s (scan with --event-store first)", d.String(), f.EventStore)
	}

	fmt.Fprintf(stderr, "%s: %d observation(s), last in run %s at %s\n",
		view.Domain, view.Observations, view.LastRunID, view.ObservedAt.Format(time.RFC3339))

	result := classify.Result{
		Domain:    view.Domain,
		Evidences: view.Evidence,
		Err:       view.Err,
		ErrKind:   view.ErrKind,
	}

	return renderResults(stdout, []classify.Result{result}, render.FilterAll, format)
}
