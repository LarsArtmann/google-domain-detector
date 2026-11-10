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
	"strings"
	"time"

	v5 "github.com/larsartmann/cmdguard/v5/pkg/cmdguard/v5"
	"github.com/larsartmann/go-output"

	"github.com/larsartmann/google-domain-detector/internal/classify"
	"github.com/larsartmann/google-domain-detector/internal/domain"
	"github.com/larsartmann/google-domain-detector/internal/input"
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
	Input      string        `flag:"input"        help:"Read domains from FILE ('-' reads stdin). Plain lists and CSV with a domain/website/url column both work" short:"i"`
	Format     string        `flag:"format"       help:"Output format"                                                                                            short:"f" default:"table"                                       values:"table,json,csv,tsv,markdown,yaml,toml,jsonl"`
	Workers    int           `flag:"workers"      help:"Parallel DNS lookups"                                                                                     short:"j" default:"8"`
	Timeout    time.Duration `flag:"timeout"      help:"Per-domain lookup budget"                                                                                           default:"5s"`
	Only       string        `flag:"only"         help:"Report only matching domains"                                                                                       default:"all"                                         values:"all,workspace,gcp,google"`
	RangesURL  string        `flag:"ranges-url"   help:"GCP IP-range feed"                                                                                                  default:"https://www.gstatic.com/ipranges/cloud.json"`
	CachePath  string        `flag:"ranges-cache" help:"Range-feed cache file (default ~/.cache/google-domain-detector/cloud.json)"                                                                                                                                    env:"GDD_RANGES_CACHE"`
	NoRanges   bool          `flag:"no-ranges"    help:"Skip GCP IP-range matching (CNAME/MX/NS evidence only)"`
	NoDKIM     bool          `flag:"no-dkim"      help:"Skip the google._domainkey probe"`
	Resolver   string        `flag:"resolver"     help:"Query this DNS server (host or host:port) instead of the system resolver"                                                                                                                                      env:"GDD_RESOLVER"`
	EventStore string        `flag:"event-store"  help:"Persist the run as events in this SQLite file (go-cqrs-lite journal; replay with the replay command)"                                                                                                          env:"GDD_EVENT_STORE"`
}

// Run builds the CLI and executes args. Construction errors are printed to
// stderr and returned; the execution error is returned unprinted because
// cmdguard reports it exactly once.
func Run(ctx context.Context, args []string, stdout, stderr io.Writer, version string) error {
	d, err := BuildCLI(stdout, stderr, version, resolve.New(), os.Stdin, stdinIsTTY())
	if err != nil {
		fmt.Fprintf(stderr, "Error: %v\n", err)

		return err
	}

	return d.ExecuteWithArgs(ctx, args)
}

// BuildCLI assembles the command tree with injected dependencies.
func BuildCLI(
	stdout, stderr io.Writer,
	version string,
	res resolve.Resolver,
	stdin io.Reader,
	stdinTTY bool,
) (*v5.CLI[RootConfig], error) {
	cli, err := v5.NewCLI(
		Name,
		"Detect which domains run Google Workspace or Google Cloud, from public DNS alone",
		RootConfig{},
		v5.WithCLIVersion(version),
	)
	if err != nil {
		return nil, err
	}

	scanCmd, cmdErr := v5.NewCommand("scan", ScanFlags{},
		func(ctx context.Context, _ *RootConfig, f ScanFlags) error {
			return runScan(ctx, f, v5.ArgsFromContext(ctx), stdout, stderr, res, stdin, stdinTTY)
		},
		v5.WithShort("Scan domains for Google Workspace and Google Cloud evidence"),
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

// runScan is the command body: collect domains, load GCP ranges, scan,
// filter, render.
func runScan(
	ctx context.Context,
	f ScanFlags,
	args []string,
	stdout, stderr io.Writer,
	res resolve.Resolver,
	stdin io.Reader,
	stdinTTY bool,
) error {
	filter, err := render.ParseFilter(f.Only)
	if err != nil {
		return err
	}

	format, err := output.ParseFormat(f.Format)
	if err != nil {
		return err
	}

	domains, skipped, err := collectDomains(args, f.Input, stdin, stdinTTY)
	if err != nil {
		return err
	}

	if len(skipped) > 0 {
		fmt.Fprintf(stderr, "skipped %d unparseable input entries (first: %q)\n", len(skipped), skipped[0])
	}

	gcp, err := loadRanges(ctx, f)
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

	finish, err := attachEventStore(ctx, f, len(domains), len(skipped), stderr, &scanOpts)
	if err != nil {
		return err
	}

	started := time.Now()

	results := scan.Run(ctx, activeRes, domains, scanOpts)

	rendered := renderResults(stdout, results, filter, format)

	if err := finish(ctx, time.Since(started), results); err != nil {
		return err
	}

	return rendered
}

// renderResults filters and writes the report in the requested format.
// json and jsonl carry the full-evidence DTO (array vs one object per
// line); every other format is table-shaped via go-output.
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
	inputDomain, skipped int,
	stderr io.Writer,
	opts *scan.Options,
) (func(context.Context, time.Duration, []classify.Result) error, error) {
	if f.EventStore == "" {
		return func(context.Context, time.Duration, []classify.Result) error { return nil }, nil
	}

	sys, err := scanstore.Open(ctx, f.EventStore)
	if err != nil {
		return nil, fmt.Errorf("open event store %s: %w", f.EventStore, err)
	}

	runID := scanstore.NewRunID()

	recorder := scanstore.NewRecorder(sys)
	if err := recorder.StartScan(ctx, runID, inputDomain, skipped, opts.Workers); err != nil {
		_ = sys.GracefulClose(context.WithoutCancel(ctx))

		return nil, fmt.Errorf("start run in event store: %w", err)
	}

	sink := scanstore.NewSink(recorder, runID, sinkDepth)
	opts.OnResult = sink.OnResult

	return func(closeCtx context.Context, duration time.Duration, results []classify.Result) error {
		if err := sink.Close(closeCtx, duration, classify.Count(results)); err != nil {
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

func collectDomains(
	args []string,
	inputPath string,
	stdin io.Reader,
	stdinTTY bool,
) ([]domain.Domain, []string, error) {
	var parsed []input.Result

	for _, raw := range args {
		parsed = append(parsed, parseLenient(strings.NewReader(raw)))
	}

	switch {
	case inputPath == "-":
		parsed = append(parsed, parseLenient(stdin))
	case inputPath != "":
		file, err := os.Open(inputPath)
		if err != nil {
			return nil, nil, fmt.Errorf("open --input %s: %w", inputPath, err)
		}
		defer func() { _ = file.Close() }()

		parsed = append(parsed, parseLenient(file))
	case len(args) == 0:
		if stdinTTY {
			return nil, nil, errors.New(
				"no domains given: pass domains as arguments, via --input FILE, or pipe a list on stdin",
			)
		}

		parsed = append(parsed, parseLenient(stdin))
	}

	seen := map[string]bool{}

	var (
		domains []domain.Domain
		skipped []string
	)
	for _, res := range parsed {
		skipped = append(skipped, res.Skipped...)
		for _, d := range res.Domains {
			if !seen[d.String()] {
				seen[d.String()] = true
				domains = append(domains, d)
			}
		}
	}

	if len(domains) == 0 {
		return nil, skipped, errors.New(
			"no valid domains given: pass domains as arguments, via --input FILE, or pipe a list on stdin",
		)
	}

	return domains, skipped, nil
}

// parseLenient parses one input source, keeping whatever it could use: a
// garbage argument degrades to a Skipped entry instead of failing the run.
func parseLenient(r io.Reader) input.Result {
	res, _ := input.Parse(r)

	return res
}

func loadRanges(ctx context.Context, f ScanFlags) (*ranges.Set, error) {
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

	set, _, err := ranges.Load(ctx, ranges.Options{URL: f.RangesURL, CachePath: cache})
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
