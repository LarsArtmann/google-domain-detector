package cli

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/larsartmann/go-output"

	"github.com/larsartmann/google-domain-detector/internal/classify"
	"github.com/larsartmann/google-domain-detector/internal/domain"
	"github.com/larsartmann/google-domain-detector/internal/render"
	"github.com/larsartmann/google-domain-detector/internal/resolve"
	"github.com/larsartmann/google-domain-detector/internal/scan"
	"github.com/larsartmann/google-domain-detector/internal/scanstore"
)

// scanCommandFile holds the scan command body: streaming input feeding,
// resume reconciliation, chunked scanning, and range loading. Split from
// cli.go so the root wiring and the scan engine each stay reviewable.

// runScan is the command body: stream domains in chunks, load GCP ranges,
// scan, filter, render. Buffered formats (table/json/csv/...) materialize
// the full report; jsonl streams each line as its domain completes.
// chunkSize <= 0 means defaultScanChunkSize; it exists so tests can exercise
// chunk boundaries without a package-global variable parallel tests could
// race on.
func runScan(
	ctx context.Context,
	f ScanFlags,
	args []string,
	stdout, stderr io.Writer,
	res resolve.Resolver,
	stdin io.Reader,
	stdinTTY bool,
	stdoutTTY bool,
	version string,
	chunkSize int,
) error {
	filter, format, tableOpts, err := scanOutputOpts(f)
	if err != nil {
		return err
	}

	ctx, budgetDone := applyRunBudget(ctx, f.Budget)
	defer budgetDone()

	resume, err := resolveResume(ctx, f)
	if err != nil {
		return err
	}

	feeder, first, last, err := scanTargets(f, args, stdin, stdinTTY, chunkSize, resume, stderr)
	if err != nil {
		return err
	}

	if feeder == nil { // --resume with nothing left to scan: the no-op was already announced
		return nil
	}

	gcp, err := loadRanges(ctx, f, version)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "warning: %v (continuing without IP-range evidence)\n", err)
	}

	activeRes := res
	if f.Resolver != "" {
		activeRes = resolve.NewWithAddr(f.Resolver)
	}

	scanOpts := scan.Options{
		Workers:   f.Workers,
		Timeout:   f.Timeout,
		GCP:       gcp,
		ProbeDKIM: !f.NoDKIM,
	}

	finish, err := attachScanRecording(ctx, f, stderr, &scanOpts, resume, feeder, first, last)
	if err != nil {
		return err
	}

	stream, finishProgress := newScanStream(ctx, format, stdoutTTY, stdout, filter, &scanOpts)
	defer finishProgress()

	started := time.Now()

	results, counts, unscanned, scanErr := scanAllChunks(ctx, feeder, first, last, scanOpts, activeRes, stream)
	if scanErr != nil {
		return scanErr
	}

	reportInputAccounting(stderr, feeder, resume)

	rendered := renderOutput(stdout, results, filter, format, tableOpts, stream)

	_, _ = fmt.Fprintf(stderr, "%d domains: workspace certain %d / likely %d, gcp %d, errors %d%s\n",
		counts.Scanned, counts.WorkspaceCertain, counts.WorkspaceLikely,
		counts.GCPCertain, counts.Errors, errorKindsSuffix(counts))

	if unscanned > 0 {
		_, _ = fmt.Fprintf(
			stderr,
			"interrupted: %d input domains were never scanned (canceled before their turn)\n",
			unscanned,
		)
	}

	if err := finish(ctx, time.Since(started), counts); err != nil {
		return err
	}

	return rendered
}

// applyRunBudget wraps ctx with the --max-runtime deadline (E31): a
// deadline on feeding, scanning, and recording. Expiry classifies in-flight
// lookups as timeouts and unstarted domains as interrupted (never
// scanned), so a budgeted run still renders, journals, and reconciles its
// coverage honestly. budget <= 0 means unlimited and returns ctx as-is.
func applyRunBudget(ctx context.Context, budget time.Duration) (context.Context, func()) {
	if budget <= 0 {
		return ctx, func() {}
	}

	budgetCtx, budgetCancel := context.WithTimeout(ctx, budget)

	return budgetCtx, budgetCancel
}

func scanTargets(
	f ScanFlags,
	args []string,
	stdin io.Reader,
	stdinTTY bool,
	chunkSize int,
	resume *scanstore.ResumeSource,
	stderr io.Writer,
) (*domainFeeder, []domain.Domain, bool, error) {
	feeder, err := newDomainFeeder(args, f.Input, stdin, stdinTTY, resumeSkipSet(resume), chunkSize)
	if err != nil {
		return nil, nil, false, err
	}

	first, last, err := feeder.next()
	if err != nil {
		return nil, nil, false, err
	}

	if len(first) > 0 {
		return feeder, first, last, nil
	}

	if resume != nil {
		_, _ = fmt.Fprintf(stderr, "resume: run %s already observed every input domain — nothing to do\n", resume.RunID)

		return nil, nil, false, nil
	}

	return nil, nil, false, usageError(errNoValidDomains)
}

// attachScanRecording journals the run start when --event-store is set and
// returns the closer that completes the run. Input totals are exact when
// everything fit in one chunk (the common case); a streamed multi-chunk run
// journals 0 = unknown, and the scan.completed tally stays the authoritative
// count. A --wave without a store is a usage error.
func attachScanRecording(
	ctx context.Context,
	f ScanFlags,
	stderr io.Writer,
	opts *scan.Options,
	resume *scanstore.ResumeSource,
	feeder *domainFeeder,
	first []domain.Domain,
	last bool,
) (func(context.Context, time.Duration, classify.Counts) error, error) {
	if f.Wave != "" && f.EventStore == "" {
		return nil, usageError(errWaveNeedsStore)
	}

	inputDomains, skipped := 0, 0

	if last {
		skippedTotal, _ := feeder.skippedEntries()
		inputDomains, skipped = len(first), skippedTotal
	}

	return attachEventStore(ctx, f, resumeRunID(resume), inputDomains, skipped, stderr, opts)
}

// scanOutputOpts parses the --only/--format/--color trio into the render
// inputs every output path shares.
func scanOutputOpts(f ScanFlags) (render.Filter, output.Format, render.TableOptions, error) {
	filter, err := render.ParseFilter(f.Only)
	if err != nil {
		return "", "", render.TableOptions{}, err
	}

	format, err := output.ParseFormat(f.Format)
	if err != nil {
		return "", "", render.TableOptions{}, usageError(err)
	}

	color, err := parseColorMode(f.Color)
	if err != nil {
		return "", "", render.TableOptions{}, err
	}

	return filter, format, render.TableOptions{ColorMode: color}, nil
}

// resolveResume loads the skip-set a --resume run starts from (nil = fresh
// run). A --resume without --event-store is a usage error, not a scan one.
// newScanStream wires the output mode onto the scan options: streaming jsonl
// (returns the stream), live progress on an interactive terminal (returns its
// finish func for the caller to defer), or neither (both no-ops/nil) for
// buffered formats on pipes.
func newScanStream(
	ctx context.Context,
	format output.Format,
	stdoutTTY bool,
	stdout io.Writer,
	filter render.Filter,
	opts *scan.Options,
) (*render.JSONLStream, func()) {
	if format == output.FormatJSONL {
		stream := render.NewJSONLStream(stdout)

		chainOnResult(opts, func(r classify.Result) {
			if render.Matches(r, filter) {
				stream.Write(r)
			}
		})

		return stream, func() {}
	}

	if stdoutTTY {
		// Live progress on an interactive terminal: pipes and jsonl streams
		// keep byte-identical output (the stream owns stdout; in-place
		// redraws would corrupt its lines). finish runs before the report
		// renders, so the final frame never fights the table for the screen.
		progress := newNomProgress(ctx, stdout)
		opts.OnDomainStart = progress.domainStarted
		chainOnResult(opts, progress.domainDone)

		return nil, func() { progress.finish(ctx) }
	}

	return nil, func() {}
}

// scanAllChunks scans the first chunk and every following feeder chunk,
// accumulating results (buffered formats only; jsonl discards per chunk),
// the verdict tally, and the count of input domains a canceled run never
// reached (placeholders the report still accounts for, but excluded from
// counts and the journal). Input totals are exact when everything fit in
// one chunk (the common case); a streamed multi-chunk run journals 0 =
// unknown, and the scan.completed tally stays the authoritative count.
func scanAllChunks(
	ctx context.Context,
	feeder *domainFeeder,
	first []domain.Domain,
	last bool,
	scanOpts scan.Options,
	activeRes resolve.Resolver,
	stream *render.JSONLStream,
) ([]classify.Result, classify.Counts, int, error) {
	var (
		counts    classify.Counts
		results   []classify.Result // buffered formats only; jsonl discards per chunk
		unscanned int
	)

	scanChunk := func(chunk []domain.Domain) {
		chunkResults := scan.Run(ctx, activeRes, chunk, scanOpts)
		if stream == nil {
			results = append(results, chunkResults...)
		}

		for i := range chunkResults {
			if chunkResults[i].Unscanned {
				unscanned++
			}

			counts.Tally(chunkResults[i])
		}
	}

	scanChunk(first)

	for !last {
		chunk, isLast, err := feeder.next()
		if err != nil {
			return nil, classify.Counts{}, 0, err
		}

		if len(chunk) == 0 {
			break
		}

		last = isLast

		scanChunk(chunk)
	}

	return results, counts, unscanned, nil
}

// chainOnResult adds next to opts.OnResult, keeping whatever callback was
// already installed (the event-store sink): both run, existing first.
func chainOnResult(opts *scan.Options, next func(classify.Result)) {
	if prev := opts.OnResult; prev != nil {
		opts.OnResult = func(r classify.Result) { prev(r); next(r) }

		return
	}

	opts.OnResult = next
}

// renderResults filters and writes the report in the requested format.
// json and jsonl carry the full-evidence DTO (array vs one object per
// line); every other format is table-shaped via go-output. The scan command
// bypasses this for jsonl, streaming lines as results complete (see
// render.JSONLStream); replay keeps the buffered path until journal walks
// stream natively.
