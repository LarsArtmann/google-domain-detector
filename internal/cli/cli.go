// Package cli wires the google-domain-detector command line: one scan
// command behind cmdguard v5 (typed flags, version/doctor/describe
// batteries). All domain logic lives in runScan, which takes injected
// dependencies and is tested directly.
package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	v5 "github.com/larsartmann/cmdguard/v5/pkg/cmdguard/v5"
	"github.com/larsartmann/go-output"

	"github.com/larsartmann/google-domain-detector/internal/classify"
	"github.com/larsartmann/google-domain-detector/internal/domain"
	"github.com/larsartmann/google-domain-detector/internal/ranges"
	"github.com/larsartmann/google-domain-detector/internal/render"
	"github.com/larsartmann/google-domain-detector/internal/resolve"
	"github.com/larsartmann/google-domain-detector/internal/scan"
	"github.com/larsartmann/google-domain-detector/internal/scanstore"
)

// Name is the binary name.
const Name = "google-domain-detector"

// RootConfig carries the root's persistent configuration (none: every flag
// belongs to the scan command).
type RootConfig struct{}

// ScanFlags is the scan command's own flag set.
//

type ScanFlags struct {
	Input      string        `flag:"input"        help:"Read domains from FILE ('-' reads stdin). Plain lists and CSV with a domain/website/url column both work"                           short:"i"`
	Format     string        `flag:"format"       help:"Output format"                                                                                                                      short:"f" default:"table"                                       values:"table,json,csv,tsv,markdown,yaml,toml,jsonl"`
	Workers    int           `flag:"workers"      help:"Parallel DNS lookups"                                                                                                               short:"j" default:"8"                                                                                                validate:"min=1,max=512"`
	Timeout    time.Duration `flag:"timeout"      help:"Per-domain lookup budget"                                                                                                                     default:"5s"`
	Only       string        `flag:"only"         help:"Report only matching domains"                                                                                                                 default:"all"                                         values:"all,workspace,gcp,google"`
	RangesURL  string        `flag:"ranges-url"   help:"GCP IP-range feed"                                                                                                                            default:"https://www.gstatic.com/ipranges/cloud.json"                                                                               env:"RANGES_URL"`
	CachePath  string        `flag:"ranges-cache" help:"Range-feed cache file (default ~/.cache/google-domain-detector/cloud.json)"                                                                                                                                                                                                       env:"RANGES_CACHE"`
	NoRanges   bool          `flag:"no-ranges"    help:"Skip GCP IP-range matching (CNAME/MX/NS evidence only)"`
	NoDKIM     bool          `flag:"no-dkim"      help:"Skip the google._domainkey probe"`
	Resolver   string        `flag:"resolver"     help:"Query this DNS server (host or host:port) instead of the system resolver"                                                                                                                                                                                                         env:"RESOLVER"`
	EventStore string        `flag:"event-store"  help:"Persist the run as events in this SQLite file (go-cqrs-lite journal; replay with the replay command)"                                                                                                                                                                             env:"EVENT_STORE"`
	Resume     string        `flag:"resume"       help:"Skip domains already observed in a recorded run (exact run ID, or 'latest'); starts a new run linked to it. Requires --event-store"`
	Budget     time.Duration `flag:"max-runtime"   help:"Abort the whole run after this budget (e.g. 2h30m); 0 = unlimited. Partial results still render, journal, and count"`
	Wave       string        `flag:"wave"         help:"Census wave ID stamped into the event journal (e.g. tranco1m-a); empty for ad-hoc runs. Requires --event-store"`
	Color      string        `flag:"color"        help:"Terminal colors for table output"                                                                                                             default:"auto"                                        values:"auto,always,never"`
}

// Run builds the CLI and executes args. Construction errors are printed to
// stderr and returned; the execution error is returned unprinted because
// cmdguard reports it exactly once. Flag-value rejections from cmdguard's
// binding layer (bad enum, duration, or validate-tag bound) are re-tagged as
// usage errors so the process exits 2 like every other bad invocation.
func Run(ctx context.Context, args []string, stdout, stderr io.Writer, version, commit string) error {
	d, err := BuildCLI(stdout, stderr, version, commit, resolve.New(), os.Stdin, stdinIsTTY(), stdoutIsTTY())
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "Error: %v\n", err)

		return err
	}

	err = executeWithUsageContract(d, ctx, args)

	return err
}

// stdoutIsTTY reports whether the process stdout is a terminal; live
// progress only renders there.
func stdoutIsTTY() bool {
	info, err := os.Stdout.Stat()
	if err != nil {
		return false
	}

	return info.Mode()&os.ModeCharDevice != 0
}

// executeWithUsageContract runs the built CLI through cmdguard and re-tags
// flag-value rejections as usage errors (exit 2). Shared by Run and the test
// seam so both entry points honor the same contract.
func executeWithUsageContract(d *v5.CLI[RootConfig], ctx context.Context, args []string) error {
	err := d.ExecuteWithArgs(ctx, args)
	if isFlagValueError(err) {
		return usageError(err)
	}

	return err
}

// isFlagValueError reports whether err comes from cmdguard's flag-binding
// validation: the value-rejection sentinel family (bad enum, duration bounds,
// validate-tag min/max/pattern). These are usage errors by definition, so Run
// re-tags them with the exit-2 contract.
func isFlagValueError(err error) bool {
	for _, sentinel := range []error{
		v5.ErrInvalidEnum,
		v5.ErrValueOutOfRange,
		v5.ErrValueTooShort,
		v5.ErrValueTooLong,
		v5.ErrValueTooSmall,
		v5.ErrValueTooLarge,
		v5.ErrValuePatternMismatch,
		v5.ErrValueEmpty,
	} {
		if errors.Is(err, sentinel) {
			return true
		}
	}

	return false
}

// cliOptions is the root option set: version/commit stamps, the GDD_ env
// prefix every env tag resolves under, and panic recovery so a bug reports
// an error instead of crashing the process.
func cliOptions(version, commit string) []v5.CLIOption {
	opts := []v5.CLIOption{
		v5.WithCLIVersion(version),
		v5.WithEnvPrefix("GDD_"),
		v5.WithMiddleware[RootConfig](v5.RecoveryMiddleware[RootConfig]()),
	}

	if commit != "" {
		opts = append(opts, v5.WithCLICommit(commit))
	}

	return opts
}

// BuildCLI assembles the command tree with injected dependencies. The
// stdin/stdout TTY flags steer interactive-only behavior (prompts, live
// progress) independently of the injected writers, which tests set to
// buffers.
func BuildCLI(
	stdout, stderr io.Writer,
	version, commit string,
	res resolve.Resolver,
	stdin io.Reader,
	stdinTTY bool,
	stdoutTTY bool,
) (*v5.CLI[RootConfig], error) {
	cli, err := v5.NewCLI(
		Name,
		"Detect which domains run Google Workspace or Google Cloud, from public DNS alone",
		RootConfig{},
		cliOptions(version, commit)...,
	)
	if err != nil {
		return nil, err
	}

	scanCmd, cmdErr := v5.NewCommand(
		"scan",
		ScanFlags{},
		func(ctx context.Context, _ *RootConfig, f ScanFlags) error {
			return runScan(ctx, f, v5.ArgsFromContext(ctx), stdout, stderr, res, stdin, stdinTTY, stdoutTTY, version, 0)
		},
		v5.WithShort("Scan domains for Google Workspace and Google Cloud evidence"),
		v5.WithExample(
			"  # Scan domains passed as arguments\n  google-domain-detector scan --format json example.com\n\n  # Scan a list on stdin, keep only Workspace hits\n  cat domains.txt | google-domain-detector scan --only workspace --format csv",
		),
	)

	replayCmd, replayErr := v5.NewCommand("replay", ReplayFlags{},
		func(ctx context.Context, _ *RootConfig, f ReplayFlags) error {
			return runReplay(ctx, f, stdout, stderr)
		},
		v5.WithShort("Rebuild a scan report from the event store, no DNS"),
	)

	showCmd, showErr := v5.NewCommand("show", ShowFlags{},
		func(ctx context.Context, _ *RootConfig, f ShowFlags) error {
			return runShow(ctx, f, v5.ArgsFromContext(ctx), stdout, stderr)
		},
		v5.WithShort("Show one domain's latest observation and verdict history from the event store"),
	)

	runsCmd, runsErr := v5.NewCommand("runs", RunsFlags{},
		func(ctx context.Context, _ *RootConfig, f RunsFlags) error {
			return runRuns(ctx, f, stdout, stderr)
		},
		v5.WithShort("List recorded scan runs from the event store"),
	)

	enrichCmd, enrichErr := v5.NewCommand("enrich", EnrichFlags{},
		func(ctx context.Context, _ *RootConfig, f EnrichFlags) error {
			return runEnrich(ctx, f, stdin, stdout, stderr)
		},
		v5.WithShort("Append workspace/gcp verdict columns to a CSV, joined from the event store (no DNS)"),
	)

	b := v5.NewBuilder(cli)
	v5.Try(b, scanCmd, cmdErr)
	v5.Try(b, replayCmd, replayErr)
	v5.Try(b, showCmd, showErr)
	v5.Try(b, runsCmd, runsErr)
	v5.Try(b, enrichCmd, enrichErr)

	// The doctor command replaces cmdguard's battery default so the
	// domain preflights (dns-resolver, ranges-feed) run alongside the DI
	// health checks; a consumer-defined "doctor" wins per the battery
	// contract.
	docCmd, docErr := v5.DoctorCommand[RootConfig](cli,
		v5.WithDoctorCheck[RootConfig]("dns-resolver", probeDNS),
		v5.WithDoctorCheck[RootConfig]("ranges-feed", probeRanges),
	)
	v5.Try(b, docCmd, docErr)

	if err := b.Build(); err != nil {
		return nil, err
	}

	return cli, nil
}

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

	// Whole-run budget (E31): a deadline on everything below — feeding,
	// scanning, recording. Expiry classifies in-flight lookups as timeouts
	// and unstarted domains as interrupted (never scanned), so a budgeted
	// run still renders, journals, and reconciles its coverage honestly.
	if f.Budget > 0 {
		var budgetCancel context.CancelFunc

		ctx, budgetCancel = context.WithTimeout(ctx, f.Budget)
		defer budgetCancel()
	}

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

	if skippedTotal, firstSkipped := feeder.skippedEntries(); skippedTotal > 0 {
		_, _ = fmt.Fprintf(stderr, "skipped %d unparseable input entries (first: %q)\n", skippedTotal, firstSkipped)
	}

	if resume != nil {
		if skipped := feeder.resumeSkippedCount(); skipped > 0 {
			_, _ = fmt.Fprintf(stderr, "resume: skipped %d input domains already observed in run %s\n", skipped, resume.RunID)
		}
	}

	rendered := renderOutput(stdout, results, filter, format, tableOpts, stream)

	_, _ = fmt.Fprintf(stderr, "%d domains: workspace certain %d / likely %d, gcp %d, errors %d%s\n",
		counts.Scanned, counts.WorkspaceCertain, counts.WorkspaceLikely,
		counts.GCPCertain, counts.Errors, errorKindsSuffix(counts))

	if unscanned > 0 {
		_, _ = fmt.Fprintf(stderr, "interrupted: %d input domains were never scanned (canceled before their turn)\n", unscanned)
	}

	if err := finish(ctx, time.Since(started), counts); err != nil {
		return err
	}

	return rendered
}

// scanTargets builds the domain feeder and pulls the first chunk. Empty
// input is a usage error; a --resume run whose input was fully observed
// already is a printed no-op, signaled by a nil feeder and nil error.
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
func resolveResume(ctx context.Context, f ScanFlags) (*scanstore.ResumeSource, error) {
	if f.Resume == "" {
		return nil, nil //nolint:nilnil // nil resume means fresh run; the caller branches on resume != nil
	}

	if f.EventStore == "" {
		return nil, usageError(errResumeNeedsStore)
	}

	source, err := scanstore.LoadResumeSource(ctx, f.EventStore, f.Resume)
	if err != nil {
		return nil, err
	}

	return &source, nil
}

func resumeSkipSet(resume *scanstore.ResumeSource) map[string]struct{} {
	if resume == nil {
		return nil
	}

	return resume.Observed
}

func resumeRunID(resume *scanstore.ResumeSource) string {
	if resume == nil {
		return ""
	}

	return resume.RunID
}

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

// renderOutput writes the report: jsonl lines were already streamed as
// results completed, so only its stream error is left to collect.
func renderOutput(
	stdout io.Writer,
	results []classify.Result,
	filter render.Filter,
	format output.Format,
	tableOpts render.TableOptions,
	stream *render.JSONLStream,
) error {
	if stream != nil {
		return stream.Err()
	}

	return renderResults(stdout, results, filter, format, tableOpts)
}

// parseColorMode resolves the --color flag, treating the zero value as auto
// (direct runScan calls skip cmdguard's flag defaults).
func parseColorMode(value string) (output.ColorMode, error) {
	if value == "" {
		value = string(output.ColorModeAuto)
	}

	mode, err := output.ParseColorMode(value)
	if err != nil {
		return mode, usageError(err)
	}

	return mode, nil
}

// errorKindsSuffix explains the error total with its kind breakdown, so a
// big tail reads as data instead of a mystery.
func errorKindsSuffix(c classify.Counts) string {
	if c.Errors == 0 {
		return ""
	}

	k := c.ErrorKinds

	return fmt.Sprintf(" (nxdomain %d, timeout %d, servfail %d, canceled %d, other %d)",
		k.NXDomain, k.Timeout, k.ServFail, k.Canceled, k.Other)
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
func renderResults(
	w io.Writer,
	results []classify.Result,
	filter render.Filter,
	format output.Format,
	opts render.TableOptions,
) error {
	results = render.Filtered(results, filter)

	switch format { //nolint:exhaustive // every registry format renders as a table; unknown shapes route via RenderUnknown
	case output.FormatJSON, output.FormatJSONL:
		// Both dispatch through go-output's unknown-data registry; render's
		// init replaced the generic serialization renderers with the
		// full-evidence DTO ones.
		return output.RenderUnknown(results, format, output.RenderOptions{Writer: w})
	default:
		return render.Table(w, results, format, opts)
	}
}

// attachEventStore, when --event-store is set, opens the event-sourced scan
// store, starts a run, and hooks observation recording into opts. It returns
// the closer that drains and completes the run; without an event store the
// closer is a no-op.
func attachEventStore(
	ctx context.Context,
	f ScanFlags,
	resumedFrom string,
	inputDomain, skipped int,
	stderr io.Writer,
	opts *scan.Options,
) (func(context.Context, time.Duration, classify.Counts) error, error) {
	if f.EventStore == "" {
		return func(context.Context, time.Duration, classify.Counts) error { return nil }, nil
	}

	sys, err := scanstore.Open(ctx, f.EventStore)
	if err != nil {
		return nil, fmt.Errorf("open event store %s: %w", f.EventStore, err)
	}

	runID := scanstore.NewRunID()

	recorder := scanstore.NewRecorder(sys)
	if err := recorder.StartScan(ctx, runID, resumedFrom, f.Wave, inputDomain, skipped, opts.Workers); err != nil {
		_ = sys.GracefulClose(context.WithoutCancel(ctx))

		return nil, fmt.Errorf("start run in event store: %w", err)
	}

	//nolint:contextcheck // record() detaches on purpose: a canceled scan still journals (resume checkpoint)
	sink := scanstore.NewSink(
		recorder,
		runID,
		sinkDepth,
	)
	opts.OnResult = sink.OnResult

	return func(closeCtx context.Context, duration time.Duration, counts classify.Counts) error {
		if err := sink.Close(closeCtx, duration, counts); err != nil {
			_, _ = fmt.Fprintf(stderr, "warning: event store recording incomplete: %v\n", err)
		}

		if err := sys.GracefulClose(context.WithoutCancel(closeCtx)); err != nil {
			_, _ = fmt.Fprintf(stderr, "warning: close event store: %v\n", err)
		}

		_, _ = fmt.Fprintf(stderr, "run %s recorded to %s (replay: %s replay --event-store %s --run %s)\n",
			runID.Get(), f.EventStore, Name, f.EventStore, runID.Get())

		return nil // recording problems are reported, never fatal to the scan report
	}, nil
}

// sinkDepth bounds the buffer between the DNS pool and the event store.
const sinkDepth = 64

// exitCodeUsage marks a bad invocation; every other failure exits 1 so
// scripts can tell "called it wrong" from "scan went wrong".
const exitCodeUsage = 2

var (
	errNoDomainsGiven = errors.New(
		"no domains given: pass domains as arguments, via --input FILE, or pipe a list on stdin",
	)
	errNoValidDomains = errors.New(
		"no valid domains given: pass domains as arguments, via --input FILE, or pipe a list on stdin",
	)
)

// usageError wraps err so the process exits 2 instead of the default 1.
func usageError(err error) error {
	exitErr, invalid := v5.NewExitError(exitCodeUsage, err)
	if invalid != nil {
		return err
	}

	return exitErr
}

func loadRanges(ctx context.Context, f ScanFlags, version string) (*ranges.Set, error) {
	if f.NoRanges {
		return nil, nil //nolint:nilnil // --no-ranges disables the set by design; absence is not an error
	}

	cache := f.CachePath
	if cache == "" {
		defaultCache, err := defaultRangesCache()
		if err != nil {
			return nil, err
		}

		cache = defaultCache
	}

	userAgent := Name
	if version != "" {
		userAgent = Name + "/" + version
	}

	set, _, err := ranges.Load(ctx, ranges.Options{URL: f.RangesURL, CachePath: cache, UserAgent: userAgent})
	if err != nil {
		return nil, err
	}

	return set, nil
}

func stdinIsTTY() bool {
	info, err := os.Stdin.Stat()
	if err != nil {
		return false
	}

	return info.Mode()&os.ModeCharDevice != 0
}
