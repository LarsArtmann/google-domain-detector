// Command google-domain-detector answers "which of these domains run Google
// Workspace or Google Cloud?" from public DNS alone.
package main

import (
	"context"
	"os"

	v5 "github.com/larsartmann/cmdguard/v5/pkg/cmdguard/v5"

	"github.com/larsartmann/google-domain-detector/internal/cli"
)

// version and commit are overridden at link time (-X main.version=...).
var (
	version = "dev"
	commit  = "" //nolint:gochecknoglobals // -X main.commit ldflags stamp; a parameter cannot reach the linker
)

func main() {
	os.Exit(v5.ExitCode(run(context.Background(), os.Args[1:])))
}

// run builds and executes the CLI against the real environment. Construction
// errors are printed here; the execution error is printed by cmdguard exactly
// once and returned unprinted.
func run(ctx context.Context, args []string) error {
	return cli.Run(ctx, args, os.Stdout, os.Stderr, version, commit)
}
