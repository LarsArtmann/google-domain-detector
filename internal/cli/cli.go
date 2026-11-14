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

	"github.com/larsartmann/google-domain-detector/internal/resolve"
)

// Name is the binary name.
const Name = "google-domain-detector"

// RootConfig carries the root's persistent configuration (none: every flag
// belongs to the scan command).
type RootConfig struct{}

// ScanFlags is the scan command's own flag set.
//

//nolint:golines // fleet golines pads/reorders tags against the project tagalign policy; tags cannot wrap (see .golangci.yml)
type ScanFlags struct {
	Input      string        `flag:"input"        help:"Read domains from FILE ('-' reads stdin). Plain lists and CSV with a domain/website/url column both work"                           short:"i"`
	Format     string        `default:"table" flag:"format" help:"Output format" short:"f" values:"table,json,csv,tsv,markdown,yaml,toml,jsonl"`
	Workers    int           `validate:"min=1,max=512" default:"8" flag:"workers" help:"Parallel DNS lookups" short:"j"`
	Timeout    time.Duration `default:"5s" flag:"timeout" help:"Per-domain lookup budget"`
	Only       string        `default:"all" flag:"only" help:"Report only matching domains" values:"all,workspace,gcp,google"`
	RangesURL  string        `default:"https://www.gstatic.com/ipranges/cloud.json" env:"RANGES_URL" flag:"ranges-url" help:"GCP IP-range feed"`
	CachePath  string        `env:"RANGES_CACHE" flag:"ranges-cache" help:"Range-feed cache file (default ~/.cache/google-domain-detector/cloud.json)"`
	NoRanges   bool          `flag:"no-ranges" help:"Skip GCP IP-range matching (CNAME/MX/NS evidence only)"`
	NoDKIM     bool          `flag:"no-dkim" help:"Skip the google._domainkey probe"`
	Resolver   string        `env:"RESOLVER" flag:"resolver" help:"Query this DNS server (host or host:port) instead of the system resolver"`
	EventStore string        `env:"EVENT_STORE" flag:"event-store" help:"Persist the run as events in this SQLite file (go-cqrs-lite journal; replay with the replay command)"`
	Resume     string        `flag:"resume" help:"Skip domains already observed in a recorded run (exact run ID, or 'latest'); starts a new run linked to it. Requires --event-store"`
	Budget     time.Duration `flag:"max-runtime" help:"Abort the whole run after this budget (e.g. 2h30m); 0 = unlimited. Partial results still render, journal, and count"`
	Wave       string        `flag:"wave" help:"Census wave ID stamped into the event journal (e.g. tranco1m-a); empty for ad-hoc runs. Requires --event-store"`
	Color      string        `default:"auto" flag:"color" help:"Terminal colors for table output" values:"auto,always,never"`
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
		v5.WithMiddleware(v5.RecoveryMiddleware[RootConfig]()),
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
	docCmd, docErr := v5.DoctorCommand(cli,
		v5.WithDoctorCheck[RootConfig]("dns-resolver", probeDNS),
		v5.WithDoctorCheck[RootConfig]("ranges-feed", probeRanges),
	)
	v5.Try(b, docCmd, docErr)

	if err := b.Build(); err != nil {
		return nil, err
	}

	return cli, nil
}

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
