package domain

import (
	"testing"
)

func TestParse(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		raw  string
		want string
	}{
		"bare domain":              {"example.com", "example.com"},
		"uppercase":                {"Example.COM", "example.com"},
		"trailing dot":             {"example.com.", "example.com"},
		"surrounding whitespace":   {"  example.com\t\n", "example.com"},
		"www prefix":               {"www.example.com", "example.com"},
		"https URL":                {"https://Shop.Example.com/pricing?x=1", "example.com"},
		"http URL bare host":       {"http://example.com", "example.com"},
		"URL without scheme":       {"example.com/about", "example.com"},
		"URL with port":            {"https://example.com:8443/", "example.com"},
		"host with port":           {"example.com:443", "example.com"},
		"userinfo":                 {"https://user@example.com/", "example.com"},
		"deep subdomain":           {"shop.example.co.uk", "shop.example.co.uk"},
		"punycode":                 {"xn--kpenhaus-0za.de", "xn--kpenhaus-0za.de"},
		"multi-level with www":     {"www.shop.example.com", "shop.example.com"},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got, err := Parse(tc.raw)
			if err != nil {
				t.Fatalf("Parse(%q) failed: %v", tc.raw, err)
			}
			if got.String() != tc.want {
				t.Errorf("Parse(%q) = %q, want %q", tc.raw, got.String(), tc.want)
			}
		})
	}
}

func TestParseErrors(t *testing.T) {
	t.Parallel()

	tests := map[string]string{
		"empty":                  "",
		"whitespace only":        "   ",
		"just a dot":             ".",
		"trailing dot only":      "com.",
		"no dot":                 "localhost",
		"www only":               "www.",
		"empty after www strip":  "www.",
		"leading hyphen":         "-example.com",
		"trailing hyphen":        "example-.com",
		"underscore label":       "ex_ample.com",
		"unicode":                "köpenhaus.de",
		"scheme only":            "https://",
		"just userinfo":          "user@",
		"label too long":         "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.com",
		"double dot":             "example..com",
		"empty between dots":     "example..com",
		"port only":              ":443",
		"ipv6 host":              "[2001:db8::1]",
		"scheme no host":         "https:///path",
		"url parse garbage":      "https://exa mple.com/",
		"unbracketed host port6": "example.com:notaport",
	}

	for name, raw := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if _, err := Parse(raw); err == nil {
				t.Errorf("Parse(%q) unexpectedly succeeded", raw)
			}
		})
	}
}

func TestParseBadPortSuffixIsRejected(t *testing.T) {
	t.Parallel()

	// "example.com:notaport" must never silently produce a garbage domain.
	if got, err := Parse("example.com:notaport"); err == nil {
		t.Fatalf("expected error, got %q", got)
	}
}

func TestHosts(t *testing.T) {
	t.Parallel()

	d, err := Parse("WWW.Example.com")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	hosts := d.Hosts()
	if len(hosts) != 2 || hosts[0] != "example.com" || hosts[1] != "www.example.com" {
		t.Errorf("Hosts() = %v, want [example.com www.example.com]", hosts)
	}
}
