package render

import (
	"bytes"
	"encoding/json/v2"
	"strings"
	"testing"

	"github.com/larsartmann/google-domain-detector/internal/classify"
)

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
