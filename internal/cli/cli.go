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
	"path/filepath"
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
type ScanFlags struct {
	Input      string        `flag:"input"        help:"Read domains from FILE ('-' reads stdin). Plain lists and CSV with a domain/website/url column both work"                           short:"i"`
	Format     string        `flag:"format"       help:"Output format"                                                                                                                      short:"f" default:"table"                                       values:"table,json,csv,tsv,markdown,yaml,toml,jsonl"`
	Workers    int           `flag:"workers"      help:"Parallel DNS lookups"                                                                                                               short:"j" default:"8"                                                                                                validate:"min=1,max=512"`
	Timeout    time.Duration `flag:"timeout"      help:"Per-domain lookup budget"                                                                                                                     default:"5s"`
	Only       string        `flag:"only"         help:"Report only matching domains"                                                                                                                 default:"all"                                         values:"all,workspace,gcp,google"`
	RangesURL  string        `flag:"ranges-url"   help:"GCP IP-range feed"                                                                                                                            default:"https://www.gstatic.com/ipranges/cloud.json"`
	CachePath  string        `flag:"ranges-cache" help:"Range-feed cache file (default ~/.cache/google-domain-detector/cloud.json)"                                                                                                                                                                                                       env:"RANGES_CACHE"`
	NoRanges   bool          `flag:"no-ranges"    help:"Skip GCP IP-range matching (CNAME/MX/NS evidence only)"`
	NoDKIM     bool          `flag:"no-dkim"      help:"Skip the google._domainkey probe"`
	Resolver   string        `flag:"resolver"     help:"Query this DNS server (host or host:port) instead of the system resolver"                                                                                                                                                                                                         env:"RESOLVER"`
	EventStore string        `flag:"event-store"  help:"Persist the run as events in this SQLite file (go-cqrs-lite journal; replay with the replay command)"                                                                                                                                                                             env:"EVENT_STORE"`
	Resume     string        `flag:"resume"       help:"Skip domains already observed in a recorded run (exact run ID, or 'latest'); starts a new run linked to it. Requires --event-store"`
	Wave       string        `flag:"wave"         help:"Census wave ID stamped into the event journal (e.g. tranco1m-a); empty for ad-hoc runs. Requires --event-store"`
}

// Run builds the CLI and executes args. Construction errors are printed to
// stderr and returned; the execution error is returned unprinted because
// cmdguard reports it exactly once.
func Run(ctx context.Context, args []string, stdout, stderr io.Writer, version, commit string) error {
	d, err := BuildCLI(stdout, stderr, version, commit, resolve.New(), os.Stdin, stdinIsTTY())
	if err != nil {
		fmt.Fprintf(stderr, "Error: %v\n", err)

		return err
	}

	return d.ExecuteWithArgs(ctx, args)
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

// BuildCLI assembles the command tree with injected dependencies.
func BuildCLI(
	stdout, stderr io.Writer,
	version, commit string,
	res resolve.Resolver,
	stdin io.Reader,
	stdinTTY bool,
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
			return runScan(ctx, f, v5.ArgsFromContext(ctx), stdout, stderr, res, stdin, stdinTTY, version)
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

	b := v5.NewBuilder(cli)
	v5.Try(b, scanCmd, cmdErr)
	v5.Try(b, replayCmd, replayErr)

	if err := b.Build(); err != nil {
		return nil, err
	}

	return cli, nil
}

// runScan is the command body: stream domains in chunks, load GCP ranges,
// scan, filter, render. Buffered formats (table/json/csv/...) materialize
// the full report; jsonl streams each line as its domain completes.
func runScan(
	ctx context.Context,
	f ScanFlags,
	args []string,
	stdout, stderr io.Writer,
	res resolve.Resolver,
	stdin io.Reader,
	stdinTTY bool,
	version string,
) error {
	filter, err := render.ParseFilter(f.Only)
	if err != nil {
		return err
	}

	format, err := output.ParseFormat(f.Format)
	if err != nil {
		return err
	}

	// A --resume run first learns what its source run already observed,
	// then skips those domains; the new run links back via scan.started.
	var (
		resume      *scanstore.ResumeSource
		skipDomains map[string]struct{}
	)

	if f.Resume != "" {
		if f.EventStore == "" {
			return usageError(errors.New("--resume requires --event-store (the journal IS the checkpoint)"))
		}

		source, err := scanstore.LoadResumeSource(ctx, f.EventStore, f.Resume)
		if err != nil {
			return err
		}

		resume = &source
		skipDomains = source.Observed
	}

	feeder, err := newDomainFeeder(args, f.Input, stdin, stdinTTY, skipDomains)
	if err != nil {
		return err
	}

	first, last, err := feeder.next()
	if err != nil {
		return err
	}

	if len(first) == 0 {
		if resume != nil {
			fmt.Fprintf(stderr, "resume: run %s already observed every input domain — nothing to do\n", resume.RunID)

			return nil
		}

		return usageError(errNoValidDomains)
	}

	gcp, err := loadRanges(ctx, f, version)
	if err != nil {
		fmt.Fprintf(stderr, "warning: %v (continuing without IP-range evidence)\n", err)
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

	// Input totals are exact when everything fit in one chunk (the common
	// case); a streamed multi-chunk run journals 0 = unknown, and the
	// scan.completed tally stays the authoritative count.
	inputDomains, skipped := 0, 0

	if last {
		skippedTotal, _ := feeder.skippedEntries()
		inputDomains, skipped = len(first), skippedTotal
	}

	resumedFrom := ""
	if resume != nil {
		resumedFrom = resume.RunID
	}

	if f.Wave != "" && f.EventStore == "" {
		return usageError(errors.New("--wave requires --event-store (waves live in the journal)"))
	}

	finish, err := attachEventStore(ctx, f, resumedFrom, inputDomains, skipped, stderr, &scanOpts)
	if err != nil {
		return err
	}

	// jsonl renders as results complete instead of after the whole run;
	// every other format needs the full result set and stays buffered.
	var stream *render.JSONLStream
	if format == output.FormatJSONL {
		stream = render.NewJSONLStream(stdout)

		chainOnResult(&scanOpts, func(r classify.Result) {
			if render.Matches(r, filter) {
				stream.Write(r)
			}
		})
	}

	started := time.Now()

	var (
		counts  classify.Counts
		results []classify.Result // buffered formats only; jsonl discards per chunk
	)

	scanChunk := func(chunk []domain.Domain) {
		chunkResults := scan.Run(ctx, activeRes, chunk, scanOpts)
		if stream == nil {
			results = append(results, chunkResults...)
		}

		for i := range chunkResults {
			counts.Tally(chunkResults[i])
		}
	}

	scanChunk(first)

	for !last {
		chunk, isLast, err := feeder.next()
		if err != nil {
			return err
		}

		if len(chunk) == 0 {
			break
		}

		last = isLast

		scanChunk(chunk)
	}

	if skippedTotal, firstSkipped := feeder.skippedEntries(); skippedTotal > 0 {
		fmt.Fprintf(stderr, "skipped %d unparseable input entries (first: %q)\n", skippedTotal, firstSkipped)
	}

	var rendered error
	if stream != nil {
		rendered = stream.Err() // lines were already written as they completed
	} else {
		rendered = renderResults(stdout, results, filter, format)
	}

	fmt.Fprintf(stderr, "%d domains: workspace certain %d / likely %d, gcp %d, errors %d%s\n",
		counts.Scanned, counts.WorkspaceCertain, counts.WorkspaceLikely,
		counts.GCPCertain, counts.Errors, errorKindsSuffix(counts))

	if err := finish(ctx, time.Since(started), counts); err != nil {
		return err
	}

	return rendered
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
func renderResults(w io.Writer, results []classify.Result, filter render.Filter, format output.Format) error {
	results = render.Filtered(results, filter)

	switch format {
	case output.FormatJSON:
		return render.JSON(w, results)
	case output.FormatJSONL:
		return render.JSONL(w, results)
	default:
		return render.Table(w, results, format)
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

	sink := scanstore.NewSink(recorder, runID, sinkDepth)
	opts.OnResult = sink.OnResult

	return func(closeCtx context.Context, duration time.Duration, counts classify.Counts) error {
		if err := sink.Close(closeCtx, duration, counts); err != nil {
			fmt.Fprintf(stderr, "warning: event store recording incomplete: %v\n", err)
		}

		if err := sys.GracefulClose(context.WithoutCancel(closeCtx)); err != nil {
			fmt.Fprintf(stderr, "warning: close event store: %v\n", err)
		}

		fmt.Fprintf(stderr, "run %s recorded to %s (replay: %s replay --event-store %s --run %s)\n",
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
		return nil, nil
	}

	cache := f.CachePath
	if cache == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("resolve cache dir: %w", err)
		}

		cache = filepath.Join(home, ".cache", Name, "cloud.json")
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
