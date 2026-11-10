// Package input reads domain lists from files or stdin: plain lists (one
// domain or URL per line, whitespace/comma/semicolon separated) or CSV with a
// recognizable header column (domain, domains, website, url, host, ...).
//
// Parsing is incremental: the Streamer yields domains line by line so
// million-row inputs never sit fully in memory. Parse remains as the
// collect-everything convenience for tests and small inputs.
package input

import (
	"bufio"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"strings"
	"unicode"

	"github.com/larsartmann/google-domain-detector/internal/domain"
)

// headerKeywords names (lowercase, quotes/spaces stripped) that mark a CSV
// column as the domain source. The first matching column, left to right, wins.
var headerKeywords = map[string]bool{
	"domain":         true,
	"domains":        true,
	"website":        true,
	"websites":       true,
	"url":            true,
	"urls":           true,
	"host":           true,
	"hostname":       true,
	"site":           true,
	"web":            true,
	"company domain": true,
}

// skippedKept bounds how many unparseable entries are retained verbatim; the
// total count is always exact, so a garbage-heavy input cannot balloon memory
// through its skip report.
const skippedKept = 5

// Result carries the parsed domains plus the entries that were skipped so the
// caller can surface "3 of 503 rows were not domains" instead of silently
// dropping them.
type Result struct {
	Domains []domain.Domain
	Skipped []string
}

// ErrNoInput reports an input source that held nothing readable at all.
var ErrNoInput = errors.New("no input: pass domains as arguments, via --input FILE, or on stdin")

// Streamer parses one input source incrementally. Mode detection (plain list
// vs header-bearing CSV) reads only the first non-empty line, so both modes
// stream line by line from any io.Reader. Duplicate domains are YIELDED, not
// filtered — cross-source first-seen dedup belongs to the caller, which sees
// every source. Skipped entries are counted exactly and capped at
// skippedKept retained examples.
type Streamer struct {
	reader   *bufio.Reader
	csv      *csv.Reader
	csvCol   int
	tokens   []string
	detected bool

	sawInput     bool
	skippedKept  []string
	skippedTotal int

	err error
}

// NewStreamer starts streaming r.
func NewStreamer(r io.Reader) *Streamer {
	return &Streamer{reader: bufio.NewReader(r)}
}

// Next yields the next parseable domain, or ok=false at end of input or on
// the first read/parse error (see Err). Invalid tokens never stop the
// stream; they land in the skip statistics.
func (s *Streamer) Next() (domain.Domain, bool) {
	if !s.detected {
		s.detected = true
		s.detectMode()
	}

	for {
		if d, ok := s.nextFromQueue(); ok {
			return d, true
		}

		if s.err != nil || !s.readMore() {
			return domain.Domain{}, false
		}
	}
}

// nextFromQueue pops the next already-tokenized domain candidate.
func (s *Streamer) nextFromQueue() (domain.Domain, bool) {
	for len(s.tokens) > 0 {
		token := s.tokens[0]
		s.tokens = s.tokens[1:]

		d, err := domain.Parse(token)
		if err != nil {
			s.skip(token)

			continue
		}

		return d, true
	}

	return domain.Domain{}, false
}

// readMore pulls one more line into the token queue (or one CSV row into the
// queue). It reports whether more input may follow.
func (s *Streamer) readMore() bool {
	if s.csv != nil {
		record, err := s.csv.Read()
		if err == io.EOF {
			return false
		}

		if err != nil {
			s.err = fmt.Errorf("parse CSV row: %w", err)

			return false
		}

		if s.csvCol < len(record) {
			s.tokens = []string{record[s.csvCol]}
		}

		return true
	}

	line, err := s.reader.ReadString('\n')
	if strings.TrimSpace(line) != "" {
		s.sawInput = true
	}

	s.tokens = tokenize(strings.TrimRight(line, "\r\n"))

	if err == io.EOF {
		return len(s.tokens) > 0
	}

	if err != nil {
		s.err = fmt.Errorf("read input: %w", err)

		return false
	}

	return true
}

// detectMode consumes leading blank lines, decides plain vs CSV from the
// first non-empty line, and re-queues that line's tokens.
func (s *Streamer) detectMode() {
	for {
		line, err := s.reader.ReadString('\n')

		trimmed := strings.TrimRight(line, "\r\n")
		if strings.TrimSpace(trimmed) == "" {
			if err != nil {
				if !errors.Is(err, io.EOF) {
					s.err = fmt.Errorf("read input: %w", err)
				}

				return
			}

			continue
		}

		s.sawInput = true

		if col, ok := csvHeaderColumn(trimmed); ok {
			s.csv = csv.NewReader(s.reader)
			s.csv.FieldsPerRecord = -1
			s.csvCol = col

			return
		}

		s.tokens = tokenize(trimmed)

		if err != nil && !errors.Is(err, io.EOF) {
			s.err = fmt.Errorf("read input: %w", err)
		}

		return
	}
}

// skip records one unparseable entry.
func (s *Streamer) skip(raw string) {
	s.skippedTotal++

	if len(s.skippedKept) < skippedKept {
		s.skippedKept = append(s.skippedKept, raw)
	}
}

// Skipped reports the exact number of unparseable entries and the first few
// verbatim examples.
func (s *Streamer) Skipped() (total int, kept []string) {
	return s.skippedTotal, s.skippedKept
}

// Err reports the first read or parse error that ended the stream.
func (s *Streamer) Err() error {
	return s.err
}

// Parse reads the whole input and returns deduplicated domains in first-seen
// order, plus skipped entries (capped at skippedKept examples; the exact
// total is the length only for inputs that skipped at most that many — small
// inputs, which is every Parse caller, keep the full list). This is the
// convenience API; product paths stream.
func Parse(r io.Reader) (Result, error) {
	s := NewStreamer(r)

	var res Result

	seen := map[string]bool{}

	for {
		d, ok := s.Next()
		if !ok {
			break
		}

		if !seen[d.String()] {
			seen[d.String()] = true
			res.Domains = append(res.Domains, d)
		}
	}

	total, kept := s.Skipped()
	res.Skipped = kept

	if s.Err() != nil {
		return res, s.Err()
	}

	if !s.sawInput {
		return res, ErrNoInput
	}

	if len(res.Domains) == 0 {
		hint := ""
		if total > 0 {
			hint = fmt.Sprintf(" (first skipped entry: %q)", kept[0])
		}

		return res, fmt.Errorf("no valid domains in input%s", hint)
	}

	return res, nil
}

func tokenize(line string) []string {
	return strings.FieldsFunc(line, func(r rune) bool {
		return unicode.IsSpace(r) || r == ',' || r == ';'
	})
}

// csvHeaderColumn reports whether the line looks like a CSV header and which
// column index carries the domains.
func csvHeaderColumn(line string) (int, bool) {
	if !strings.Contains(line, ",") {
		return 0, false
	}

	for i, field := range splitCSVLine(line) {
		if headerKeywords[normalizeHeader(field)] {
			return i, true
		}
	}

	return 0, false
}

func splitCSVLine(line string) []string {
	cr := csv.NewReader(strings.NewReader(line))
	cr.FieldsPerRecord = -1

	fields, err := cr.Read()
	if err != nil {
		return strings.Split(line, ",")
	}

	return fields
}

func normalizeHeader(field string) string {
	return strings.ToLower(strings.TrimSpace(strings.Trim(field, `"' `)))
}
