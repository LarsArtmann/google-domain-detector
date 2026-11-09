// Package cli wires the google-domain-detector command line: one scan
// command behind cmdguard v5 (typed flags, version/doctor/describe
// batteries). All domain logic lives in runScan, which takes injected
// dependencies and is tested directly.
package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	v5 "github.com/larsartmann/cmdguard/v5/pkg/cmdguard/v5"
	"github.com/larsartmann/go-output"

	"github.com/larsartmann/google-domain-detector/internal/domain"
	"github.com/larsartmann/google-domain-detector/internal/input"
	"github.com/larsartmann/google-domain-detector/internal/ranges"
	"github.com/larsartmann/google-domain-detector/internal/render"
	"github.com/larsartmann/google-domain-detector/internal/resolve"
	"github.com/larsartmann/google-domain-detector/internal/scan"
)

// Name is the binary name.
const Name = "google-domain-detector"

// RootConfig carries the root's persistent configuration (none: every flag
// belongs to the scan command).
type RootConfig struct{}

// ScanFlags is the scan command's own flag set.
type ScanFlags struct {
	Input     string        `flag:"input" short:"i" help:"Read domains from FILE ('-' reads stdin). Plain lists and CSV with a domain/website/url column both work"`
	Format    string        `flag:"format" short:"f" default:"table" values:"table,json,csv,tsv,markdown" help:"Output format"`
	Workers   int           `flag:"workers" short:"j" default:"8" help:"Parallel DNS lookups"`
	Timeout   time.Duration `flag:"timeout" default:"5s" help:"Per-domain lookup budget"`
	Only      string        `flag:"only" default:"all" values:"all,workspace,gcp,google" help:"Report only matching domains"`
	RangesURL string        `flag:"ranges-url" default:"https://www.gstatic.com/ipranges/cloud.json" help:"GCP IP-range feed"`
	CachePath string        `flag:"ranges-cache" env:"GDD_RANGES_CACHE" help:"Range-feed cache file (default ~/.cache/google-domain-detector/cloud.json)"`
	NoRanges  bool          `flag:"no-ranges" help:"Skip GCP IP-range matching (CNAME/MX/NS evidence only)"`
	NoDKIM    bool          `flag:"no-dkim" help:"Skip the google._domainkey probe"`
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
func BuildCLI(stdout, stderr io.Writer, version string, res resolve.Resolver, stdin io.Reader, stdinTTY bool) (*v5.CLI[RootConfig], error) {
	cli, err := v5.NewCLI(Name, "Detect which domains run Google Workspace or Google Cloud, from public DNS alone", RootConfig{}, v5.WithCLIVersion(version))
	if err != nil {
		return nil, err
	}

	scanCmd, cmdErr := v5.NewCommand("scan", ScanFlags{},
		func(ctx context.Context, _ *RootConfig, f ScanFlags) error {
			return runScan(ctx, f, v5.ArgsFromContext(ctx), stdout, stderr, res, stdin, stdinTTY)
		},
		v5.WithShort("Scan domains for Google Workspace and Google Cloud evidence"),
	)

	b := v5.NewBuilder(cli)
	v5.Try(b, scanCmd, cmdErr)
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

	results := scan.Run(ctx, res, domains, scan.Options{
		Workers:   f.Workers,
		Timeout:   f.Timeout,
		GCP:       gcp,
		ProbeDKIM: !f.NoDKIM,
	})

	results = render.Filtered(results, filter)
	if format == output.FormatJSON {
		return render.JSON(stdout, results)
	}
	return render.Table(stdout, results, format)
}

func collectDomains(args []string, inputPath string, stdin io.Reader, stdinTTY bool) ([]domain.Domain, []string, error) {
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
			return nil, nil, fmt.Errorf("no domains given: pass domains as arguments, via --input FILE, or pipe a list on stdin")
		}
		parsed = append(parsed, parseLenient(stdin))
	}

	seen := map[string]bool{}
	var domains []domain.Domain
	var skipped []string
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
		return nil, skipped, fmt.Errorf("no valid domains given")
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
