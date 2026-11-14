package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/larsartmann/go-cqrs-lite/system/v4"
	"github.com/larsartmann/go-output"

	"github.com/larsartmann/google-domain-detector/internal/classify"
	"github.com/larsartmann/google-domain-detector/internal/render"
	"github.com/larsartmann/google-domain-detector/internal/scanstore"
)

// ReplayFlags is the replay command's flag set.
//

//nolint:golines // fleet golines pads/reorders tags against the project tagalign policy; tags cannot wrap (see .golangci.yml)
type ReplayFlags struct {
	EventStore    string `env:"EVENT_STORE" flag:"event-store" help:"SQLite event store written by scan --event-store"`
	Run           string `flag:"run" help:"Rebuild this run ID (default: the most recent run)"`
	Since         string `flag:"since" help:"Diff mode: report only domains whose verdict flipped since this run ID (the adoption/departure signal)"`
	AuditExcluded string `flag:"audit-excluded" help:"Supersession audit: list every domain whose last journal observation comes from one of these comma-separated run IDs or wave names (bare or wave:NAME; the poison check; --run/--since/--only must be unset)"`
	Format        string `default:"table" flag:"format" help:"Output format" values:"table,json,csv,tsv,markdown,yaml,toml,jsonl"`
	Only          string `default:"all" flag:"only" help:"Report only matching domains" values:"all,workspace,gcp,google"`
	Color         string `default:"auto" flag:"color" help:"Terminal colors for table output" values:"auto,always,never"`
}

// runSummaryLine is the one-line run readout shared by scan and replay:
// verdict totals plus the error-kind breakdown, with wave/ruleset provenance
// when the journal carries it.
func runSummaryLine(prefix string, c classify.Counts) string {
	var b strings.Builder

	_, _ = fmt.Fprintf(&b, "%s workspace certain %d / likely %d, gcp %d, errors %d%s",
		prefix, c.WorkspaceCertain, c.WorkspaceLikely, c.GCPCertain, c.Errors, errorKindsSuffix(c))

	return b.String()
}

// provenanceParts bounds the wave/ruleset stamp list; both stamps may
// appear on one run line.
const provenanceParts = 2

// provenanceSuffix renders the wave/ruleset journal stamps, empty when the
// run predates them (pre-census journals decode with zero values).
func provenanceSuffix(wave string, ruleset classify.Ruleset) string {
	parts := make([]string, 0, provenanceParts)
	if wave != "" {
		parts = append(parts, "wave "+wave)
	}

	if ruleset != "" {
		parts = append(parts, "ruleset "+string(ruleset))
	}

	if len(parts) == 0 {
		return ""
	}

	return " [" + strings.Join(parts, ", ") + "]"
}

// runReplay rebuilds a scan report from the event journal alone: no DNS.
func runReplay(ctx context.Context, f ReplayFlags, stdout, stderr io.Writer) error {
	if f.EventStore == "" {
		return usageError(errEventStoreRequired)
	}

	if f.AuditExcluded != "" {
		return runReplayAudit(ctx, f, stdout, stderr)
	}

	filter, err := render.ParseFilter(f.Only)
	if err != nil {
		return err
	}

	format, err := output.ParseFormat(f.Format)
	if err != nil {
		return err
	}

	colorMode, err := parseColorMode(f.Color)
	if err != nil {
		return err
	}

	sys, err := scanstore.Boot(ctx, f.EventStore)
	if err != nil {
		return fmt.Errorf("open event store %s: %w", f.EventStore, err)
	}

	defer func() { _ = sys.Close() }()

	// jsonl streams straight out of the journal walk — a whole-TLD store
	// never materializes; every other format rebuilds fully first. Diff mode
	// (--since) is the one exception on the memory promise: the baseline run
	// is materialized to make flips comparable, the walked run still streams.
	if format == output.FormatJSONL {
		return runReplayStreaming(ctx, f, sys, stdout, stderr, filter)
	}

	return runReplayBuffered(ctx, f, sys, stdout, stderr, filter, format, colorMode)
}

// mapReplayError rewrites journal-read failures into user-facing errors:
// an empty journal names the store, an unknown run names the run and store.
func mapReplayError(err error, f ReplayFlags) error {
	if errors.Is(err, scanstore.ErrNoRuns) {
		return fmt.Errorf("%s: %w", f.EventStore, err)
	}

	if errors.Is(err, scanstore.ErrUnknownRun) {
		return fmt.Errorf("run %q not found in %s: %w", runRefOrLatest(f.Run), f.EventStore, err)
	}

	return err
}

// runReplayStreaming is the jsonl path: matching (delta mode: flipping)
// observations are written as the journal walk reaches them.
func runReplayStreaming(
	ctx context.Context,
	f ReplayFlags,
	sys *system.System,
	stdout, stderr io.Writer,
	filter render.Filter,
) error {
	var (
		fromByDomain map[string]classify.Result
		fromSummary  *scanstore.RunSummary
	)

	if f.Since != "" {
		from, err := replayBaseline(ctx, sys, f.Since, f.EventStore)
		if err != nil {
			return err
		}

		fromByDomain = from.byDomain()
		fromSummary = from.summary()
	}

	stream := render.NewJSONLStream(stdout)

	flips := 0

	summary, err := scanstore.StreamObservations(ctx, sys, f.Run, func(r classify.Result) error {
		if render.Matches(r, filter) && isFlipAgainst(fromByDomain, r) {
			stream.Write(r)

			flips++
		}

		return nil
	})
	if err != nil {
		return mapReplayError(err, f)
	}

	_, _ = fmt.Fprintf(stderr, "run %s: %d domains, %s%s\n",
		summary.RunID, summary.Counts.Scanned,
		runSummaryLine("", summary.Counts),
		provenanceSuffix(summary.Wave, summary.Ruleset))

	if f.Since != "" {
		_, _ = fmt.Fprintf(stderr, "delta vs run %s: %d flips\n", f.Since, flips)
		warnRulesetMismatch(stderr, fromSummary, &summary)
	}

	return stream.Err()
}

// runReplayBuffered is every non-jsonl path: the run rebuilds fully, delta
// mode filters it down to flips, then one report renders.
func runReplayBuffered(
	ctx context.Context,
	f ReplayFlags,
	sys *system.System,
	stdout, stderr io.Writer,
	filter render.Filter,
	format output.Format,
	colorMode output.ColorMode,
) error {
	replay, err := scanstore.ReplayJournal(ctx, sys, f.Run)
	if err != nil {
		return mapReplayError(err, f)
	}

	summary := runSummary(replay)

	if summary != nil {
		_, _ = fmt.Fprintf(stderr, "run %s: %d domains, %s%s\n",
			summary.RunID, summary.Counts.Scanned,
			runSummaryLine("", summary.Counts),
			provenanceSuffix(summary.Wave, summary.Ruleset))
	}

	results := replay.Results

	if f.Since != "" {
		delta, err := applyDelta(ctx, f, sys, stderr, replay, summary)
		if err != nil {
			return err
		}

		results = delta
	}

	return renderResults(stdout, results, filter, format, render.TableOptions{ColorMode: colorMode})
}

// runSummary locates the summary of the replay's selected run; nil when the
// journal predates scan.started summaries.
func runSummary(replay scanstore.Replay) *scanstore.RunSummary {
	for i := range replay.Runs {
		if replay.Runs[i].RunID == replay.RunID {
			return &replay.Runs[i]
		}
	}

	return nil
}

// runRefOrLatest names the run a not-found error refers to; the empty
// filter means "latest", which cannot be unknown once any run exists.
func runRefOrLatest(run string) string {
	if run == "" {
		return "latest"
	}

	return run
}

// warnRulesetMismatch flags delta comparisons across detection-rule
// changes: flips between different rulesets may be rule artifacts, not
// adoption. Pre-census journals (empty ruleset) never warn.
func warnRulesetMismatch(stderr io.Writer, from, to *scanstore.RunSummary) {
	if from == nil || to == nil {
		return
	}

	if from.Ruleset != "" && to.Ruleset != "" && from.Ruleset != to.Ruleset {
		_, _ = fmt.Fprintf(
			stderr,
			"warning: ruleset changed between runs (%s → %s); flips may be rule artifacts, not adoption\n",
			from.Ruleset,
			to.Ruleset,
		)
	}
}
