package cli

import (
	"context"
	"fmt"
	"io"

	"github.com/larsartmann/google-domain-detector/internal/scanstore"
)

// resumeFile reconciles a --resume run with the journal checkpoint: which
// run it continues, which domains to skip, and the input accounting report.

// reportInputAccounting prints where input domains went besides the scan:
// unparseable entries and domains a --resume skipped as already observed.
func reportInputAccounting(stderr io.Writer, feeder *domainFeeder, resume *scanstore.ResumeSource) {
	if skippedTotal, firstSkipped := feeder.skippedEntries(); skippedTotal > 0 {
		_, _ = fmt.Fprintf(stderr, "skipped %d unparseable input entries (first: %q)\n", skippedTotal, firstSkipped)
	}

	if resume != nil {
		if skipped := feeder.resumeSkippedCount(); skipped > 0 {
			_, _ = fmt.Fprintf(
				stderr,
				"resume: skipped %d input domains already observed in run %s\n",
				skipped,
				resume.RunID,
			)
		}
	}
}

// scanTargets builds the domain feeder and pulls the first chunk. Empty
// input is a usage error; a --resume run whose input was fully observed
// already is a printed no-op, signaled by a nil feeder and nil error.
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
