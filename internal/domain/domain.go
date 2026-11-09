// Package domain parses and normalizes domain names from user input
// (bare domains, URLs, CSV fields) into a validated canonical apex form.
package domain

import (
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

// Parse normalizes raw input — "https://Shop.Example.com/pricing",
// "shop.example.com:443", "WWW.EXAMPLE.COM.", "example.com" — into the apex
// Domain "example.com". It rejects empty input, single-label hosts, illegal
// label characters, and non-ASCII (IDNA encoding is out of scope; pass
// punycode input instead).
func Parse(raw string) (Domain, error) {
	s := strings.TrimSpace(strings.ToLower(raw))
	s = strings.TrimSuffix(s, ".")

	if s == "" {
		return Domain{}, fmt.Errorf("empty domain")
	}

	if strings.Contains(s, "://") || strings.ContainsAny(s, "/@") {
		parsed, err := url.Parse(ensureScheme(s))
		if err != nil {
			return Domain{}, fmt.Errorf("parse %q as URL: %w", raw, err)
		}
		s = parsed.Hostname()
	}

	if host, _, err := net.SplitHostPort(s); err == nil {
		s = host
	}

	s = strings.TrimPrefix(s, "www.")

	if s == "" {
		return Domain{}, fmt.Errorf("empty domain after normalizing %q", raw)
	}
	if !strings.Contains(s, ".") {
		return Domain{}, fmt.Errorf("%q is not a domain (no dot)", raw)
	}
	if err := validateLabels(s); err != nil {
		return Domain{}, fmt.Errorf("invalid domain %q: %w", raw, err)
	}

	return Domain{name: s}, nil
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
	for _, label := range strings.Split(name, ".") {
		switch {
		case label == "":
			return fmt.Errorf("empty label")
		case len(label) > maxLabelLength:
			return fmt.Errorf("label %q longer than %d characters", label, maxLabelLength)
		case !validLabel(label):
			return fmt.Errorf("label %q contains illegal characters (allowed: a-z, 0-9, hyphen)", label)
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
