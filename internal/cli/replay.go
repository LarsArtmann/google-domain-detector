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
	AuditExcluded string `flag:"audit-excluded" help:"Supersession audit: list every domain whose last journal observation comes from one of these comma-separated run IDs (the poison check; --run/--since/--only must be unset)"`
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

// applyDelta narrows replay results to the domains that flipped since the
// baseline run and reports the flip tallies plus any ruleset mismatch.
func applyDelta(
	ctx context.Context,
	f ReplayFlags,
	sys *system.System,
	stderr io.Writer,
	replay scanstore.Replay,
	summary *scanstore.RunSummary,
) ([]classify.Result, error) {
	from, err := replayBaseline(ctx, sys, f.Since, f.EventStore)
	if err != nil {
		return nil, err
	}

	flips := scanstore.FlipDiff(from.replay.Results, replay.Results)

	flipSet := make(map[string]bool, len(flips))

	for _, flip := range flips {
		flipSet[flip.Domain] = true
	}

	results := make([]classify.Result, 0, len(flips))

	for _, r := range replay.Results {
		if flipSet[r.Domain] {
			results = append(results, r)
		}
	}

	counts := scanstore.CountFlips(flips)
	_, _ = fmt.Fprintf(stderr, "delta vs run %s: %d flips — workspace adopted %d / left %d, gcp adopted %d / left %d\n",
		f.Since, len(flips),
		counts.WorkspaceAdopted, counts.WorkspaceLeft,
		counts.GCPAdopted, counts.GCPLeft)

	warnRulesetMismatch(stderr, from.summary(), summary)

	return results, nil
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

// baseline is the --since side of a delta replay.
type baseline struct {
	replay scanstore.Replay
}

func (b baseline) byDomain() map[string]classify.Result {
	byDomain := make(map[string]classify.Result, len(b.replay.Results))
	for _, r := range b.replay.Results {
		byDomain[r.Domain] = r
	}

	return byDomain
}

func (b baseline) summary() *scanstore.RunSummary {
	return runSummary(b.replay)
}

func replayBaseline(ctx context.Context, sys *system.System, sinceRun, eventStore string) (baseline, error) {
	replay, err := scanstore.ReplayJournal(ctx, sys, sinceRun)
	if err != nil {
		if errors.Is(err, scanstore.ErrNoRuns) {
			return baseline{}, fmt.Errorf("%s: %w", eventStore, err)
		}

		if errors.Is(err, scanstore.ErrUnknownRun) {
			return baseline{}, fmt.Errorf("--since run %q not found in %s: %w", sinceRun, eventStore, err)
		}

		return baseline{}, err
	}

	return baseline{replay: replay}, nil
}

// runRefOrLatest names the run a not-found error refers to; the empty
// filter means "latest", which cannot be unknown once any run exists.
func runRefOrLatest(run string) string {
	if run == "" {
		return "latest"
	}

	return run
}

// isFlipAgainst reports whether the observation differs from the baseline
// state for its domain; without a baseline (--since unset) everything passes.
func isFlipAgainst(fromByDomain map[string]classify.Result, to classify.Result) bool {
	if fromByDomain == nil {
		return true
	}

	from, seen := fromByDomain[to.Domain]
	if !seen {
		return false
	}

	flips := scanstore.FlipDiff([]classify.Result{from}, []classify.Result{to})

	return len(flips) == 1
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

// runReplayAudit is the --audit-excluded mode: the supersession poison
// check. It walks the whole journal and reports the domains whose last-wins
// observation derives from an excluded run, so a correction pass's
// "everything superseded" claim is proven per domain instead of assumed.
// Findings are data, not failure: the exit code stays 0 and gates assert
// on the stderr count.
func runReplayAudit(ctx context.Context, f ReplayFlags, stdout, stderr io.Writer) error {
	if err := validateAuditFlags(f); err != nil {
		return err
	}

	excluded := parseRunList(f.AuditExcluded)

	format, err := output.ParseFormat(f.Format)
	if err != nil {
		return err
	}

	colorMode, err := parseColorMode(f.Color)
	if err != nil {
		return err
	}

	audit, sys, err := auditJournalFindings(ctx, f.EventStore, excluded)
	if err != nil {
		return err
	}

	defer func() { _ = sys.Close() }()

	_, _ = fmt.Fprintf(stderr, "supersession audit: %d of %d domain(s) last observed by an excluded run%s\n",
		len(audit.Findings), audit.TotalDomains, byRunSuffix(excluded, audit.Findings))

	table := output.NewTable([]string{"Run", "Domain", "Error kind", "Error"})

	for _, finding := range audit.Findings {
		row := []string{finding.RunID, finding.Domain, string(finding.ErrKind), finding.Err}
		if err := table.AddRowChecked(row); err != nil {
			return fmt.Errorf("add audit row for %s: %w", finding.Domain, err)
		}
	}

	// Footer is a human decoration: data formats (csv/tsv/yaml/toml) must
	// stay pure rows — go-output renders whatever is set, so gate it here.
	if format == output.FormatTable || format == output.FormatMarkdown {
		table.SetFooter([]string{
			fmt.Sprintf("%d poisoned of %d domain(s)", len(audit.Findings), audit.TotalDomains), "", "", "",
		})
	}

	return output.RenderTable(table, format, output.RenderOptions{
		Writer:    stdout,
		Title:     "Supersession Audit",
		ColorMode: colorMode,
	})
}

// validateAuditFlags rejects mode conflicts and an empty run list before
// any I/O: the audit reports the whole journal's last-wins state, so run
// selectors have no meaning here.
func validateAuditFlags(f ReplayFlags) error {
	if f.Run != "" || f.Since != "" || (f.Only != "" && f.Only != "all") {
		return usageError(errAuditExcludedCombination)
	}

	if len(parseRunList(f.AuditExcluded)) == 0 {
		return usageError(fmt.Errorf("%w: %q", errAuditExcludedEmpty, f.AuditExcluded))
	}

	return nil
}

// auditJournalFindings boots the journal-only reader and runs the
// supersession audit, mapping the scanstore sentinels to user-facing
// failures the way the replay run modes do. The caller closes the system.
func auditJournalFindings(
	ctx context.Context,
	eventStore string,
	excluded []string,
) (scanstore.SupersessionAudit, *system.System, error) {
	sys, err := scanstore.Boot(ctx, eventStore)
	if err != nil {
		return scanstore.SupersessionAudit{}, nil, fmt.Errorf("open event store %s: %w", eventStore, err)
	}

	audit, err := scanstore.AuditSupersession(ctx, sys, excluded)

	switch {
	case err == nil:
		return audit, sys, nil
	case errors.Is(err, scanstore.ErrNoRuns):
		return scanstore.SupersessionAudit{}, nil, fmt.Errorf("%s: %w", eventStore, err)
	case errors.Is(err, scanstore.ErrUnknownRun):
		return scanstore.SupersessionAudit{}, nil, fmt.Errorf("excluded run not found in %s: %w", eventStore, err)
	default:
		return scanstore.SupersessionAudit{}, nil, err
	}
}

// byRunSuffix summarizes the findings per excluded run in the order the
// runs were asked for, so summary lines diff cleanly between audits.
func byRunSuffix(excluded []string, findings []scanstore.SupersessionFinding) string {
	if len(findings) == 0 {
		return ""
	}

	counts := make(map[string]int, len(excluded))
	for _, finding := range findings {
		counts[finding.RunID]++
	}

	parts := make([]string, 0, len(excluded))
	for _, runID := range excluded {
		parts = append(parts, fmt.Sprintf("%s=%d", runID, counts[runID]))
	}

	return " (" + strings.Join(parts, ", ") + ")"
}

// parseRunList splits a comma-separated run-ID list: trimmed, empty parts
// dropped, duplicates collapsed in first-seen order.
func parseRunList(raw string) []string {
	seen := make(map[string]struct{})

	var runs []string

	for part := range strings.SplitSeq(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		if _, dup := seen[part]; dup {
			continue
		}

		seen[part] = struct{}{}
		runs = append(runs, part)
	}

	return runs
}
