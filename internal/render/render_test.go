package render

import (
	"bytes"
	"encoding/json/v2"
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

func TestJSONLOneObjectPerLine(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	if err := JSONL(&buf, sampleResults()); err != nil {
		t.Fatalf("JSONL failed: %v", err)
	}

	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	if len(lines) != len(sampleResults()) {
		t.Fatalf("want %d lines, got %d:\n%s", len(sampleResults()), len(lines), buf.String())
	}

	if !strings.Contains(lines[0], `"domain":"acme.com"`) && !strings.Contains(lines[0], `"domain": "acme.com"`) {
		t.Errorf("first jsonl line must carry the first domain:\n%s", lines[0])
	}
}

func TestJSONRoundTrip(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	if err := JSON(&buf, sampleResults()); err != nil {
		t.Fatalf("JSON failed: %v", err)
	}

	var decoded []struct {
		Domain    string `json:"domain"`
		Workspace struct {
			Detected   bool   `json:"detected"`
			Confidence string `json:"confidence"`
		} `json:"workspace"`
		GCP struct {
			Detected bool `json:"detected"`
		} `json:"gcp"`
		Evidences []classify.Evidence `json:"evidence"`
		Error     string              `json:"error"`
	}
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, buf.String())
	}

	if len(decoded) != 4 {
		t.Fatalf("decoded %d results, want 4", len(decoded))
	}

	acme := decoded[0]
	if !acme.Workspace.Detected || acme.Workspace.Confidence != "certain" {
		t.Errorf("acme workspace = %+v", acme.Workspace)
	}

	if !acme.GCP.Detected {
		t.Error("acme gcp must be detected")
	}

	if len(acme.Evidences) != 2 {
		t.Errorf("acme evidence count = %d, want 2", len(acme.Evidences))
	}

	beta := decoded[1]
	if !beta.Workspace.Detected || beta.Workspace.Confidence != "likely" {
		t.Errorf("beta workspace = %+v", beta.Workspace)
	}

	plain := decoded[2]
	if plain.Workspace.Detected || plain.GCP.Detected {
		t.Error("plain.net must be undetected everywhere")
	}

	gone := decoded[3]
	if gone.Error == "" {
		t.Error("gone.example must carry its error")
	}
}

func TestJSONUndetectedVerdictIsCompact(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	if err := JSON(&buf, []classify.Result{{Domain: "plain.net"}}); err != nil {
		t.Fatal(err)
	}
	// Machine-oriented single-line JSON: jq-friendly, no pretty printing.
	if !strings.Contains(buf.String(), `"workspace":{"detected":false}`) {
		t.Errorf("undetected verdict must serialize compactly, got:\n%s", buf.String())
	}
}

func TestEvidenceSummary(t *testing.T) {
	t.Parallel()

	r := classify.Result{
		Domain: "acme.com",
		Evidences: []classify.Evidence{
			{Kind: classify.EvidenceMXGoogle, Host: "acme.com", Record: "smtp.google.com"},
			{Kind: classify.EvidenceCNAMERunApp, Host: "www.acme.com", Record: "x.a.run.app"},
		},
	}
	got := EvidenceSummary(r)

	want := "mx_google:smtp.google.com; run_app:www→x.a.run.app"
	if got != want {
		t.Errorf("EvidenceSummary = %q, want %q", got, want)
	}
}
