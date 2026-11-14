package render

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/larsartmann/go-output"

	"github.com/larsartmann/google-domain-detector/internal/classify"
)

func sampleResults() []classify.Result {
	return []classify.Result{
		{
			Domain: "acme.com",
			Evidences: []classify.Evidence{
				{
					Kind:     classify.EvidenceMXGoogle,
					Platform: classify.PlatformWorkspace,
					Weight:   classify.WeightStrong,
					Host:     "acme.com",
					Record:   "smtp.google.com",
				},
				{
					Kind:     classify.EvidenceCNAMEFirebase,
					Platform: classify.PlatformGCP,
					Weight:   classify.WeightStrong,
					Host:     "www.acme.com",
					Record:   "acme.web.app",
				},
			},
		},
		{
			Domain: "beta.io",
			Evidences: []classify.Evidence{
				{
					Kind:     classify.EvidenceSPFGoogle,
					Platform: classify.PlatformWorkspace,
					Weight:   classify.WeightWeak,
					Host:     "beta.io",
					Record:   "v=spf1 include:_spf.google.com ~all",
				},
			},
		},
		{Domain: "plain.net"},
		{Domain: "gone.example", Err: "domain not found"},
	}
}

func TestParseFilter(t *testing.T) {
	t.Parallel()

	for _, ok := range []string{"all", "workspace", "gcp", "google"} {
		if _, err := ParseFilter(ok); err != nil {
			t.Errorf("ParseFilter(%q) failed: %v", ok, err)
		}
	}

	if _, err := ParseFilter("everything"); err == nil {
		t.Error("ParseFilter must reject unknown filters")
	}
}

func TestFiltered(t *testing.T) {
	t.Parallel()

	results := sampleResults()

	workspace := Filtered(results, FilterWorkspace)
	if len(workspace) != 2 { // acme (certain) + beta (likely)
		t.Errorf("workspace filter: got %d, want 2", len(workspace))
	}

	gcp := Filtered(results, FilterGCP)
	if len(gcp) != 1 || gcp[0].Domain != "acme.com" {
		t.Errorf("gcp filter: got %v", domainsOf(gcp))
	}

	all := Filtered(results, FilterAll)
	if len(all) != 4 {
		t.Errorf("all filter: got %d, want 4", len(all))
	}
}

func domainsOf(rs []classify.Result) []string {
	out := make([]string, 0, len(rs))
	for _, r := range rs {
		out = append(out, r.Domain)
	}

	return out
}

func TestTableRendering(t *testing.T) {
	t.Parallel()

	for _, format := range []string{"csv", "tsv", "markdown"} {
		f, err := output.ParseFormat(format)
		if err != nil {
			t.Fatalf("format %s: %v", format, err)
		}

		var buf bytes.Buffer
		if err := Table(&buf, sampleResults(), f, TableOptions{}); err != nil {
			t.Fatalf("Table(%s) failed: %v", format, err)
		}

		out := buf.String()
		for _, want := range []string{"acme.com", "certain", "acme.web.app", "gone.example", "domain not found"} {
			if !strings.Contains(out, want) {
				t.Errorf("%s output missing %q", format, want)
			}
		}

		if strings.Contains(out, "\x1b[") {
			t.Errorf("%s output must not embed ANSI codes when writing to a buffer", format)
		}
	}
}

func TestTableFooterOnlyInHumanFormats(t *testing.T) {
	t.Parallel()

	results := sampleResults()

	workspace, gcp := 0, 0

	for _, r := range results {
		if r.WorkspaceVerdict().Detected {
			workspace++
		}

		if r.GCPVerdict().Detected {
			gcp++
		}
	}

	total := fmt.Sprintf("%d domains", len(results))

	for _, format := range []string{"table", "markdown"} {
		f, err := output.ParseFormat(format)
		if err != nil {
			t.Fatalf("format %s: %v", format, err)
		}

		var buf bytes.Buffer
		if err := Table(&buf, results, f, TableOptions{}); err != nil {
			t.Fatalf("Table(%s) failed: %v", format, err)
		}

		out := buf.String()
		for _, want := range []string{total, fmt.Sprintf("%d workspace", workspace), fmt.Sprintf("%d gcp", gcp)} {
			if !strings.Contains(out, want) {
				t.Errorf("%s output missing footer %q:\n%s", format, want, out)
			}
		}
	}

	for _, format := range []string{"csv", "tsv"} {
		f, err := output.ParseFormat(format)
		if err != nil {
			t.Fatalf("format %s: %v", format, err)
		}

		var buf bytes.Buffer
		if err := Table(&buf, results, f, TableOptions{}); err != nil {
			t.Fatalf("Table(%s) failed: %v", format, err)
		}

		if out := buf.String(); strings.Contains(out, total) {
			t.Errorf("%s must stay pure data (no totals row):\n%s", format, out)
		}
	}
}

func TestTableTitleReachesMarkdown(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer

	f, err := output.ParseFormat("markdown")
	if err != nil {
		t.Fatalf("markdown: %v", err)
	}

	if err := Table(&buf, sampleResults(), f, TableOptions{Title: "Acme Scan Report"}); err != nil {
		t.Fatalf("Table(markdown) failed: %v", err)
	}

	if !strings.Contains(buf.String(), "Acme Scan Report") {
		t.Errorf("markdown output missing the title:\n%s", buf.String())
	}
}

func TestTableColorModeHonored(t *testing.T) {
	t.Parallel()

	var alwaysBuf, neverBuf bytes.Buffer
	if err := Table(
		&alwaysBuf,
		sampleResults(),
		output.FormatTable,
		TableOptions{ColorMode: output.ColorModeAlways},
	); err != nil {
		t.Fatalf("Table(always) failed: %v", err)
	}

	if err := Table(
		&neverBuf,
		sampleResults(),
		output.FormatTable,
		TableOptions{ColorMode: output.ColorModeNever},
	); err != nil {
		t.Fatalf("Table(never) failed: %v", err)
	}

	if !strings.Contains(alwaysBuf.String(), "\x1b[") {
		t.Errorf("color=always must emit ANSI codes:\n%s", alwaysBuf.String())
	}

	if strings.Contains(neverBuf.String(), "\x1b[") {
		t.Errorf("color=never must suppress ANSI codes:\n%s", neverBuf.String())
	}
}

func TestTableSerializationFormats(t *testing.T) {
	t.Parallel()

	for _, format := range []string{"yaml", "toml"} {
		f, err := output.ParseFormat(format)
		if err != nil {
			t.Fatalf("format %s: %v", format, err)
		}

		var buf bytes.Buffer
		if err := Table(&buf, sampleResults(), f, TableOptions{}); err != nil {
			t.Fatalf("Table(%s) failed: %v", format, err)
		}

		out := buf.String()
		for _, want := range []string{"acme.com", "certain", "acme.web.app", "gone.example", "domain not found"} {
			if !strings.Contains(out, want) {
				t.Errorf("%s output missing %q:\n%s", format, want, out)
			}
		}
	}
}
