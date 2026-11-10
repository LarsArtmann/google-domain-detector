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

// defaultScanChunkSize bounds how many domains (and, for buffered formats,
// their results) sit in memory at once. Every chunk is full except the last
// one — a short chunk marks the end of the input. Tests shrink it through
// runScan's chunkSize parameter instead of a package global, so parallel
// tests cannot race on it.
const defaultScanChunkSize = 4096

// domainFeeder yields deduplicated domain chunks across all input sources in
// first-seen order: positional arguments first, then the --input file or
// piped stdin. Skipped entries are aggregated exactly (first example kept).
type domainFeeder struct {
	streamers []*input.Streamer
	closers   []io.Closer
	current   int
	seen      map[string]bool
	skip      map[string]struct{}
	chunkSize int

	skippedTotal int
	skippedFirst string
}

// newDomainFeeder wires up the input sources. skip names domains a
// --resume run must not re-scan (already observed in its source run);
// chunkSize <= 0 means defaultScanChunkSize. It fails before reading
// anything when no source exists at all (no arguments, no --input, and a
// terminal stdin nobody will pipe into).
func newDomainFeeder(
	args []string,
	inputPath string,
	stdin io.Reader,
	stdinTTY bool,
	skip map[string]struct{},
	chunkSize int,
) (*domainFeeder, error) {
	if inputPath == "" && len(args) == 0 && stdinTTY {
		return nil, usageError(errNoDomainsGiven)
	}

	if chunkSize <= 0 {
		chunkSize = defaultScanChunkSize
	}

	feeder := &domainFeeder{seen: map[string]bool{}, skip: skip, chunkSize: chunkSize}

	for _, raw := range args {
		feeder.streamers = append(feeder.streamers, input.NewStreamer(strings.NewReader(raw)))
		feeder.closers = append(feeder.closers, nil)
	}

	switch {
	case inputPath == "-":
		feeder.streamers = append(feeder.streamers, input.NewStreamer(stdin))
		feeder.closers = append(feeder.closers, nil)
	case inputPath != "":
		file, err := os.Open(inputPath)
		if err != nil {
			return nil, fmt.Errorf("open --input %s: %w", inputPath, err)
		}

		feeder.streamers = append(feeder.streamers, input.NewStreamer(file))
		feeder.closers = append(feeder.closers, file)
	case len(args) == 0:
		// Piped stdin (a terminal was already rejected above).
		feeder.streamers = append(feeder.streamers, input.NewStreamer(stdin))
		feeder.closers = append(feeder.closers, nil)
	}

	return feeder, nil
}

// next yields the next chunk. last reports that the input is drained after
// this chunk, so callers can pin exact input totals up front.
func (f *domainFeeder) next() (chunk []domain.Domain, last bool, err error) {
	chunk = make([]domain.Domain, 0, f.chunkSize)

	for f.current < len(f.streamers) && len(chunk) < f.chunkSize {
		if drainErr := f.drainStreamer(f.current, &chunk); drainErr != nil {
			return nil, false, drainErr
		}
	}

	return chunk, f.current == len(f.streamers), nil
}

// drainStreamer appends new (unskipped, unseen) domains from streamer index
// i until the chunk is full or the stream ends; an exhausted stream is
// closed, its skipped-entry tally folded in, and the feeder advances to the
// next source.
func (f *domainFeeder) drainStreamer(i int, chunk *[]domain.Domain) error {
	streamer := f.streamers[i]

	for len(*chunk) < f.chunkSize {
		dom, ok := streamer.Next()
		if !ok {
			if streamErr := streamer.Err(); streamErr != nil {
				return streamErr
			}

			total, kept := streamer.Skipped()
			f.foldSkipped(total, kept)

			if closer := f.closers[i]; closer != nil {
				_ = closer.Close()
			}

			f.current++

			return nil
		}

		if f.accept(dom) {
			*chunk = append(*chunk, dom)
		}
	}

	return nil
}

func (f *domainFeeder) foldSkipped(total int, kept []string) {
	f.skippedTotal += total

	if f.skippedFirst == "" && len(kept) > 0 {
		f.skippedFirst = kept[0]
	}
}

// accept reports whether dom enters the scan: skip-listed (--resume) and
// already-seen domains never do, but both mark the domain as seen so the
// first source wins.
func (f *domainFeeder) accept(dom domain.Domain) bool {
	if f.skip != nil {
		if _, done := f.skip[dom.String()]; done {
			f.seen[dom.String()] = true

			return false
		}
	}

	if !f.seen[dom.String()] {
		f.seen[dom.String()] = true

		return true
	}

	return false
}

// skippedEntries reports the aggregated skip count and first example.
func (f *domainFeeder) skippedEntries() (int, string) {
	return f.skippedTotal, f.skippedFirst
}
