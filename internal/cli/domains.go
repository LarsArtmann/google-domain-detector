// Domain collection: streaming, chunked, cross-source. Input parsing never
// materializes the whole list — domains flow to the scanner in chunks of
// scanChunkSize, deduplicated first-seen across every source (arguments,
// then --input FILE or stdin).

package cli

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/larsartmann/google-domain-detector/internal/domain"
	"github.com/larsartmann/google-domain-detector/internal/input"
)

// scanChunkSize bounds how many domains (and, for buffered formats, their
// results) sit in memory at once. Every chunk is full except the last one —
// a short chunk marks the end of the input. A var so tests can exercise
// chunk boundaries without 4096-domain fixtures.
var scanChunkSize = 4096

// domainFeeder yields deduplicated domain chunks across all input sources in
// first-seen order: positional arguments first, then the --input file or
// piped stdin. Skipped entries are aggregated exactly (first example kept).
type domainFeeder struct {
	streamers []*input.Streamer
	closers   []io.Closer
	current   int
	seen      map[string]bool

	skippedTotal int
	skippedFirst string
}

// newDomainFeeder wires up the input sources. It fails before reading
// anything when no source exists at all (no arguments, no --input, and a
// terminal stdin nobody will pipe into).
func newDomainFeeder(args []string, inputPath string, stdin io.Reader, stdinTTY bool) (*domainFeeder, error) {
	if inputPath == "" && len(args) == 0 && stdinTTY {
		return nil, usageError(errNoDomainsGiven)
	}

	f := &domainFeeder{seen: map[string]bool{}}

	for _, raw := range args {
		f.streamers = append(f.streamers, input.NewStreamer(strings.NewReader(raw)))
		f.closers = append(f.closers, nil)
	}

	switch {
	case inputPath == "-":
		f.streamers = append(f.streamers, input.NewStreamer(stdin))
		f.closers = append(f.closers, nil)
	case inputPath != "":
		file, err := os.Open(inputPath)
		if err != nil {
			return nil, fmt.Errorf("open --input %s: %w", inputPath, err)
		}

		f.streamers = append(f.streamers, input.NewStreamer(file))
		f.closers = append(f.closers, file)
	case len(args) == 0:
		// Piped stdin (a terminal was already rejected above).
		f.streamers = append(f.streamers, input.NewStreamer(stdin))
		f.closers = append(f.closers, nil)
	}

	return f, nil
}

// next yields the next chunk. last reports that the input is drained after
// this chunk, so callers can pin exact input totals up front.
func (f *domainFeeder) next() (chunk []domain.Domain, last bool, err error) {
	chunk = make([]domain.Domain, 0, scanChunkSize)

	for f.current < len(f.streamers) && len(chunk) < scanChunkSize {
		s := f.streamers[f.current]

		for len(chunk) < scanChunkSize {
			d, ok := s.Next()
			if !ok {
				if streamErr := s.Err(); streamErr != nil {
					return nil, false, streamErr
				}

				total, kept := s.Skipped()
				f.skippedTotal += total
				if f.skippedFirst == "" && len(kept) > 0 {
					f.skippedFirst = kept[0]
				}

				if closer := f.closers[f.current]; closer != nil {
					_ = closer.Close()
				}

				f.current++

				break
			}

			if !f.seen[d.String()] {
				f.seen[d.String()] = true
				chunk = append(chunk, d)
			}
		}
	}

	return chunk, f.current == len(f.streamers), nil
}

// skippedEntries reports the aggregated skip count and first example.
func (f *domainFeeder) skippedEntries() (int, string) {
	return f.skippedTotal, f.skippedFirst
}
