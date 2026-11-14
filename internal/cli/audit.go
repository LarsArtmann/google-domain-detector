package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/larsartmann/go-cqrs-lite/system/v4"
	"github.com/larsartmann/go-output"

	"github.com/larsartmann/google-domain-detector/internal/scanstore"
)

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

	excluded, err := resolveAuditExcluded(ctx, sys, f.AuditExcluded)
	if err != nil {
		return mapAuditError(err, f.EventStore)
	}

	audit, err := scanstore.AuditSupersession(ctx, sys, excluded)
	if err != nil {
		return mapAuditError(err, f.EventStore)
	}

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

// mapAuditError names the event store on the audit's journal-failure
// paths. The audit runs on the whole store, so every failure answers
// "which store" first; ErrUnknownRun here is belt and braces (the token
// resolver above only admits journal-known IDs, so it fires only on a
// resolver/audit disagreement).
func mapAuditError(err error, store string) error {
	switch {
	case errors.Is(err, scanstore.ErrNoRuns):
		return fmt.Errorf("%s: %w", store, err)
	case errors.Is(err, scanstore.ErrUnknownRun):
		return fmt.Errorf("excluded run not found in %s: %w", store, err)
	default:
		return err
	}
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

// resolveAuditExcluded expands each comma-separated token into run IDs via
// the journal's run summaries: "wave:NAME" expands to every run stamped
// with that wave, a bare token matching a run ID stays as-is, and a bare
// token matching a wave name expands like wave:NAME (wave names cannot
// collide with run IDs). Summaries are first-started order, so wave
// expansion reads chronologically; anything unknown is a usage error
// naming the token. Journal-only by design: the audit must stay independent
// of the projection read models (Boot-composed, no host).
func resolveAuditExcluded(ctx context.Context, sys *system.System, raw string) ([]string, error) {
	runs, err := scanstore.RunSummaries(ctx, sys)
	if err != nil {
		return nil, err
	}

	runsByID := make(map[string]struct{}, len(runs))
	runsByWave := make(map[string][]string)

	for _, run := range runs {
		runsByID[run.RunID] = struct{}{}

		if run.Wave != "" {
			runsByWave[run.Wave] = append(runsByWave[run.Wave], run.RunID)
		}
	}

	resolved := make([]string, 0, len(runsByID))
	seen := make(map[string]struct{}, len(runsByID))

	add := func(id string) {
		if _, dup := seen[id]; dup {
			return
		}

		seen[id] = struct{}{}
		resolved = append(resolved, id)
	}

	for token := range strings.SplitSeq(raw, ",") {
		token = strings.TrimSpace(token)
		if token == "" {
			continue
		}

		wave, isWave := strings.CutPrefix(token, "wave:")
		if !isWave {
			if _, isID := runsByID[token]; isID {
				add(token)

				continue
			}

			wave = token
		}

		ids, known := runsByWave[wave]
		if !known {
			return nil, usageError(fmt.Errorf("%w: %q", errAuditExcludedUnknown, token))
		}

		for _, id := range ids {
			add(id)
		}
	}

	return resolved, nil
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
