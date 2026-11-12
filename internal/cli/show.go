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
//

type ShowFlags struct {
	EventStore string `env:"EVENT_STORE" flag:"event-store" help:"SQLite event store written by scan --event-store"`
	Format     string `                  flag:"format"      help:"Output format"                                    default:"table" values:"table,json,csv,tsv,markdown,yaml,toml,jsonl"`
	Color      string `                  flag:"color"       help:"Terminal colors for table output"                 default:"auto"  values:"auto,always,never"`
}

var errShowNeedsOneDomain = errors.New("show needs exactly one domain: show example.com --event-store scans.db")

// runShow prints one domain's latest observation from the read model: the
// verdicts its evidence implies, how often the store has observed it, and
// which run saw it last. No DNS, no journal walk.
func runShow(ctx context.Context, f ShowFlags, args []string, stdout, stderr io.Writer) error {
	if f.EventStore == "" {
		return usageError(errEventStoreRequired)
	}

	if len(args) != 1 {
		return usageError(errShowNeedsOneDomain)
	}

	dom, err := domain.Parse(args[0])
	if err != nil {
		return usageError(fmt.Errorf("not a domain: %q: %w", args[0], err))
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

	view, found, err := scanstore.LatestObservation(ctx, sys, dom.String())
	if err != nil {
		return err
	}

	if !found {
		return fmt.Errorf("%s in %s: %w", dom.String(), f.EventStore, errNoObservations)
	}

	_, _ = fmt.Fprintf(stderr, "%s: %d observation(s), last in run %s at %s\n",
		view.Domain, view.Observations, view.LastRunID, view.ObservedAt.Format(time.RFC3339))

	result := classify.Result{
		Domain:    view.Domain,
		Evidences: view.Evidence,
		Err:       view.Err,
		ErrKind:   view.ErrKind,
	}

	return renderResults(
		stdout,
		[]classify.Result{result},
		render.FilterAll,
		format,
		render.TableOptions{ColorMode: colorMode},
	)
}
