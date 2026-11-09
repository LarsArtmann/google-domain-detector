// Package input reads domain lists from files or stdin: plain lists (one
// domain or URL per line, whitespace/comma/semicolon separated) or CSV with a
// recognizable header column (domain, domains, website, url, host, ...).
package input

import (
	"bufio"
	"encoding/csv"
	"fmt"
	"io"
	"strings"
	"unicode"

	"github.com/larsartmann/google-domain-detector/internal/domain"
)

// headerKeywords names (lowercase, quotes/spaces stripped) that mark a CSV
// column as the domain source. The first matching column, left to right, wins.
var headerKeywords = map[string]bool{
	"domain":        true,
	"domains":       true,
	"website":       true,
	"websites":      true,
	"url":           true,
	"urls":          true,
	"host":          true,
	"hostname":      true,
	"site":          true,
	"web":           true,
	"company domain": true,
}

// Result carries the parsed domains plus the entries that were skipped so the
// caller can surface "3 of 503 rows were not domains" instead of silently
// dropping them.
type Result struct {
	Domains []domain.Domain
	Skipped []string
}

// Parse reads the whole input and returns domains in first-seen order.
// The first non-empty line decides the mode: if it contains a comma and one
// of its fields is a known header keyword, the input is treated as CSV and
// the first matching column is read; otherwise every whitespace/comma/
// semicolon-separated token of every line is parsed as a domain.
// Invalid tokens are skipped and reported in Result.Skipped — a lead list
// with one garbage row must not lose the other 500 rows.
func Parse(r io.Reader) (Result, error) {
	reader := bufio.NewReader(r)
	first, err := firstNonEmptyLine(reader)
	if err != nil {
		return Result{}, fmt.Errorf("read input: %w", err)
	}
	if first == "" {
		return Result{}, fmt.Errorf("no input: pass domains as arguments, via --input FILE, or on stdin")
	}

	var res Result
	seen := map[string]bool{}

	add := func(raw string) {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			return
		}
		d, err := domain.Parse(raw)
		if err != nil {
			res.Skipped = append(res.Skipped, raw)
			return
		}
		if !seen[d.String()] {
			seen[d.String()] = true
			res.Domains = append(res.Domains, d)
		}
	}

	if col, ok := csvHeaderColumn(first); ok {
		cr := csv.NewReader(reader)
		cr.FieldsPerRecord = -1

		rowNum := 1
		for {
			record, err := cr.Read()
			if err == io.EOF {
				break
			}
			if err != nil {
				return res, fmt.Errorf("parse CSV row %d: %w", rowNum, err)
			}
			rowNum++
			if col < len(record) {
				add(record[col])
			}
		}
	} else {
		addTokens(first, add)
		for {
			line, err := reader.ReadString('\n')
			addTokens(strings.TrimRight(line, "\r\n"), add)
			if err == io.EOF {
				break
			}
			if err != nil {
				return res, fmt.Errorf("read input: %w", err)
			}
		}
	}

	if len(res.Domains) == 0 {
		hint := ""
		if len(res.Skipped) > 0 {
			hint = fmt.Sprintf(" (first skipped entry: %q)", res.Skipped[0])
		}
		return res, fmt.Errorf("no valid domains in input%s", hint)
	}

	return res, nil
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

func addTokens(line string, add func(string)) {
	for _, token := range strings.FieldsFunc(line, func(r rune) bool {
		return unicode.IsSpace(r) || r == ',' || r == ';'
	}) {
		add(token)
	}
}

func firstNonEmptyLine(r *bufio.Reader) (string, error) {
	for {
		line, err := r.ReadString('\n')
		trimmed := strings.TrimRight(line, "\r\n")
		if strings.TrimSpace(trimmed) != "" {
			return trimmed, nil
		}
		if err == io.EOF {
			return "", nil
		}
		if err != nil {
			return "", err
		}
	}
}
