package input_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/larsartmann/google-domain-detector/internal/input"
)

// FuzzParse throws arbitrary bytes at the input parser: malformed CSV
// quoting, mixed encodings, hostile header keywords, NULs, and truncated
// multi-byte sequences must never panic. Errors are allowed only for the
// usage case the parser owns (no valid domains at all); bad ROWS are
// skipped and reported, never fatal (AGENTS input contract).
func FuzzParse(f *testing.F) {
	seeds := []string{
		"example.com\nexample.org\n",
		"example.com, example.org\n",
		"name,domain\nAcme,acme.com\n",
		"\"quoted\",\"header\"\nacme.com\n",
		"domain,url,email\nacme.com,https://acme.com/x,a@acme.com\n",
		"host;hostname;site\nacme.com\n",
		"xn--80ak6aa92e.com\n",
		"\x00binary\x01junk\n",
		"strings with\ttabs\n",
		"trailing,comma,\n,\n,\n",
		"weird\\\"quote\"escape\n",
		"\xff\xfe\x00bom-utf16-ish\n",
		"UPDATED_HEADER\ndomain,host\nacme.com\n",
		strings.Repeat("a,b,c,d,e\n", 500),
	}

	for _, seed := range seeds {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, raw string) {
		result, _ := input.Parse(strings.NewReader(raw))

		for _, d := range result.Domains {
			if strings.ContainsRune(d.String(), '\x00') {
				t.Fatalf("domain %q retains a NUL from input %q", d.String(), raw)
			}
		}
	})
}

// TestFuzzCorpusOnDisk keeps committed crashers executable: every file
// under testdata/fuzz-corpus is parsed and must obey the same contract.
func TestFuzzCorpusOnDisk(t *testing.T) {
	t.Parallel()

	corpus := filepath.Join("testdata", "fuzz-corpus")

	entries, err := os.ReadDir(corpus)
	if err != nil {
		t.Skipf("no corpus dir: %v", err)
	}

	for _, entry := range entries {
		t.Run(entry.Name(), func(t *testing.T) {
			t.Parallel()

			raw, err := os.ReadFile(filepath.Join(corpus, entry.Name()))
			if err != nil {
				t.Fatalf("read corpus file: %v", err)
			}

			if _, err := input.Parse(strings.NewReader(string(raw))); err != nil {
				t.Fatalf("Parse must never hard-fail on bad input, got: %v", err)
			}
		})
	}
}
