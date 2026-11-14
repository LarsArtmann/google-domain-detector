package cli

import (
	"fmt"
	"io"

	"github.com/larsartmann/go-output"

	"github.com/larsartmann/google-domain-detector/internal/classify"
	"github.com/larsartmann/google-domain-detector/internal/render"
)

// outputFile holds the shared report-rendering helpers used by scan and
// replay output paths.

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
