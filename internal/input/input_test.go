package input

import (
	"strings"
	"testing"
)

func names(r Result) []string {
	out := make([]string, 0, len(r.Domains))
	for _, d := range r.Domains {
		out = append(out, d.String())
	}
	return out
}

func TestParsePlainList(t *testing.T) {
	t.Parallel()

	in := "example.com\nwww.shop.example.org\nhttps://artmann.tech/en\n"
	res, err := Parse(strings.NewReader(in))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	want := []string{"example.com", "shop.example.org", "artmann.tech"}
	if strings.Join(names(res), ",") != strings.Join(want, ",") {
		t.Errorf("got %v, want %v", names(res), want)
	}
}

func TestParsePlainListCommaAndSpaceSeparated(t *testing.T) {
	t.Parallel()

	in := "example.com, shop.example.org;artmann.tech"
	res, err := Parse(strings.NewReader(in))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	want := []string{"example.com", "shop.example.org", "artmann.tech"}
	if strings.Join(names(res), ",") != strings.Join(want, ",") {
		t.Errorf("got %v, want %v", names(res), want)
	}
}

func TestParseDeduplicatesPreservingFirstSeenOrder(t *testing.T) {
	t.Parallel()

	in := "b.example\na.example\nB.Example.\nwww.a.example\n"
	res, err := Parse(strings.NewReader(in))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	want := []string{"b.example", "a.example"}
	if strings.Join(names(res), ",") != strings.Join(want, ",") {
		t.Errorf("got %v, want %v", names(res), want)
	}
}

func TestParseCSVWithDomainColumn(t *testing.T) {
	t.Parallel()

	in := `Name,Website,Employees
"ACME GmbH",https://www.acme.de,50
Beta Ltd,beta.co.uk,10
Gamma,,,5
`
	res, err := Parse(strings.NewReader(in))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	want := []string{"acme.de", "beta.co.uk"}
	if strings.Join(names(res), ",") != strings.Join(want, ",") {
		t.Errorf("got %v, want %v", names(res), want)
	}
}

func TestParseCSVFirstMatchingColumnWins(t *testing.T) {
	t.Parallel()

	// Both "host" and "url" match; host is further left and must win even
	// though the url column also carries domains.
	in := `host,url
mail.example.com,https://other.example.org
`
	res, err := Parse(strings.NewReader(in))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if len(res.Domains) != 1 || res.Domains[0].String() != "mail.example.com" {
		t.Errorf("got %v, want [mail.example.com]", names(res))
	}
}

func TestParseCSVQuotedHeader(t *testing.T) {
	t.Parallel()

	in := "\"Company\",'Domain'\nACME,acme.de\n"
	res, err := Parse(strings.NewReader(in))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if len(res.Domains) != 1 || res.Domains[0].String() != "acme.de" {
		t.Errorf("got %v, want [acme.de]", names(res))
	}
}

func TestParsePlainListWithCommasButNoHeaderKeywords(t *testing.T) {
	t.Parallel()

	// Commas present, but the first line has no header keyword: plain mode.
	in := "alpha, beta, gamma\nexample.com, example.org"
	res, err := Parse(strings.NewReader(in))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if len(res.Skipped) != 3 { // alpha, beta, gamma
		t.Errorf("skipped = %v, want 3 entries", res.Skipped)
	}
	want := []string{"example.com", "example.org"}
	if strings.Join(names(res), ",") != strings.Join(want, ",") {
		t.Errorf("got %v, want %v", names(res), want)
	}
}

func TestParseSkipsInvalidEntries(t *testing.T) {
	t.Parallel()

	in := "example.com\nnot a domain\n...\nexample.org"
	res, err := Parse(strings.NewReader(in))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if len(res.Domains) != 2 {
		t.Errorf("domains = %v, want 2", names(res))
	}
	if len(res.Skipped) != 2 {
		t.Errorf("skipped = %v, want 2", res.Skipped)
	}
}

func TestParseEmptyInput(t *testing.T) {
	t.Parallel()

	if _, err := Parse(strings.NewReader("")); err == nil {
		t.Error("expected error for empty input")
	}
	if _, err := Parse(strings.NewReader("\n  \n\n")); err == nil {
		t.Error("expected error for whitespace-only input")
	}
}

func TestParseNoValidDomains(t *testing.T) {
	t.Parallel()

	if _, err := Parse(strings.NewReader("hello world")); err == nil {
		t.Error("expected error when no token parses as a domain")
	}
}
