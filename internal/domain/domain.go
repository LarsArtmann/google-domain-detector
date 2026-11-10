// Package domain parses and normalizes domain names from user input
// (bare domains, URLs, CSV fields) into a validated canonical apex form.
package domain

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
)

const maxLabelLength = 63

// Domain is a validated, normalized domain name: lowercase ASCII, no scheme,
// no path, no port, no leading "www", no trailing dot.
type Domain struct {
	name string
}

// Parse-failure cores. Callers match with errors.Is; contextual detail
// (the raw input, the offending label) is wrapped around them.
var (
	ErrEmptyDomain   = errors.New("empty domain")
	ErrNotADomain    = errors.New("not a domain (no dot)")
	ErrEmptyLabel    = errors.New("empty label")
	ErrLabelTooLong  = errors.New("label longer than the maximum")
	ErrBadLabelChars = errors.New("label contains illegal characters (allowed: a-z, 0-9, hyphen)")
)

// Parse normalizes raw input — "https://Shop.Example.com/pricing",
// "shop.example.com:443", "WWW.EXAMPLE.COM.", "example.com" — into the apex
// Domain "example.com". It rejects empty input, single-label hosts, illegal
// label characters, and non-ASCII (IDNA encoding is out of scope; pass
// punycode input instead).
func Parse(raw string) (Domain, error) {
	normalized := strings.TrimSpace(strings.ToLower(raw))
	normalized = strings.TrimSuffix(normalized, ".")

	if normalized == "" {
		return Domain{}, ErrEmptyDomain
	}

	if strings.Contains(normalized, "://") || strings.ContainsAny(normalized, "/@") {
		parsed, err := url.Parse(ensureScheme(normalized))
		if err != nil {
			return Domain{}, fmt.Errorf("parse %q as URL: %w", raw, err)
		}

		normalized = parsed.Hostname()
	}

	if host, _, err := net.SplitHostPort(normalized); err == nil {
		normalized = host
	}

	normalized = strings.TrimPrefix(normalized, "www.")

	if normalized == "" {
		return Domain{}, fmt.Errorf("%q: %w", raw, ErrEmptyDomain)
	}

	if !strings.Contains(normalized, ".") {
		return Domain{}, fmt.Errorf("%q: %w", raw, ErrNotADomain)
	}

	if err := validateLabels(normalized); err != nil {
		return Domain{}, fmt.Errorf("invalid domain %q: %w", raw, err)
	}

	return Domain{name: normalized}, nil
}

// String returns the canonical apex name.
func (d Domain) String() string { return d.name }

// Hosts returns the hostnames to probe for this domain: the apex first, then
// "www." — apex+www is the minimum pair that catches hosting set up on either.
func (d Domain) Hosts() []string {
	return []string{d.name, "www." + d.name}
}

func ensureScheme(s string) string {
	if !strings.Contains(s, "://") {
		return "http://" + s
	}

	return s
}

func validateLabels(name string) error {
	for label := range strings.SplitSeq(name, ".") {
		switch {
		case label == "":
			return ErrEmptyLabel
		case len(label) > maxLabelLength:
			return fmt.Errorf("label %q longer than %d characters: %w", label, maxLabelLength, ErrLabelTooLong)
		case !validLabel(label):
			return fmt.Errorf("label %q: %w", label, ErrBadLabelChars)
		}
	}

	return nil
}

func validLabel(label string) bool {
	for i, r := range label {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
		case r == '-':
			if i == 0 || i == len(label)-1 {
				return false
			}
		default:
			return false
		}
	}

	return true
}
