// Package render turns scan results into the supported output formats.
// Table-shaped formats (table, csv, tsv, markdown) go through go-output's
// registry; JSON is serialized from a richer DTO so verdicts and evidence
// survive a round trip.
package render

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/larsartmann/go-output"
	_ "github.com/larsartmann/go-output/delimited"     // registers csv, tsv
	_ "github.com/larsartmann/go-output/markdown"      // registers markdown
	_ "github.com/larsartmann/go-output/serialization" // registers json, xml, yaml, jsonl, toml
	_ "github.com/larsartmann/go-output/table"         // registers the terminal table

	"github.com/larsartmann/google-domain-detector/internal/classify"
)

// Filter selects which results are reported.
type Filter string

const (
	FilterAll       Filter = "all"
	FilterWorkspace Filter = "workspace"
	FilterGCP       Filter = "gcp"
	FilterGoogle    Filter = "google"
)

// ParseFilter validates a user-supplied filter name.
func ParseFilter(s string) (Filter, error) {
	switch Filter(s) {
	case FilterAll, FilterWorkspace, FilterGCP, FilterGoogle:
		return Filter(s), nil
	}
	return Filter(""), fmt.Errorf("unknown filter %q (want all, workspace, gcp, or google)", s)
}

// Matches reports whether the result passes the filter.
func Matches(r classify.Result, f Filter) bool {
	switch f {
	case FilterWorkspace:
		return r.WorkspaceVerdict().Detected
	case FilterGCP:
		return r.GCPVerdict().Detected
	case FilterGoogle:
		return r.WorkspaceVerdict().Detected || r.GCPVerdict().Detected || r.GenericGoogle()
	default:
		return true
	}
}

// Filtered returns the results passing the filter.
func Filtered(results []classify.Result, f Filter) []classify.Result {
	out := make([]classify.Result, 0, len(results))
	for _, r := range results {
		if Matches(r, f) {
			out = append(out, r)
		}
	}
	return out
}

// Table renders results in the given table-shaped format (validated against
// go-output's format enum by the caller).
func Table(w io.Writer, results []classify.Result, format output.Format) error {
	data := output.NewTable([]string{"Domain", "Workspace", "GCP", "Evidence", "Error"})
	for _, r := range results {
		data.AddRow([]string{
			r.Domain,
			verdictCell(r.WorkspaceVerdict()),
			verdictCell(r.GCPVerdict()),
			EvidenceSummary(r),
			r.Err,
		})
	}
	return output.RenderTable(data, format, output.RenderOptions{Writer: w})
}

// JSON writes the full-fidelity results: verdicts expanded, every evidence
// item with kind/platform/weight/host/record.
func JSON(w io.Writer, results []classify.Result) error {
	dto := make([]jsonResult, 0, len(results))
	for _, r := range results {
		dto = append(dto, jsonResult{
			Domain:        r.Domain,
			Workspace:     jsonVerdictFrom(r.WorkspaceVerdict()),
			GCP:           jsonVerdictFrom(r.GCPVerdict()),
			GenericGoogle: r.GenericGoogle(),
			Evidences:     r.Evidences,
			Error:         r.Err,
		})
	}

	enc := json.NewEncoder(w)
	return enc.Encode(dto)
}

type jsonResult struct {
	Domain        string              `json:"domain"`
	Workspace     jsonVerdict         `json:"workspace"`
	GCP           jsonVerdict         `json:"gcp"`
	GenericGoogle bool                `json:"google_other_evidence"`
	Evidences     []classify.Evidence `json:"evidence"`
	Error         string              `json:"error,omitempty"`
}

type jsonVerdict classify.Verdict

func jsonVerdictFrom(v classify.Verdict) jsonVerdict {
	return jsonVerdict{Detected: v.Detected, Confidence: v.Confidence}
}

func (v jsonVerdict) MarshalJSON() ([]byte, error) {
	if !v.Detected {
		return []byte(`{"detected":false}`), nil
	}
	return json.Marshal(struct {
		Detected   bool                `json:"detected"`
		Confidence classify.Confidence `json:"confidence"`
	}{Detected: true, Confidence: v.Confidence})
}

// EvidenceSummary packs a result's evidence into one table cell:
// "mx:smtp.google.com; run_app:www→xxx.a.run.app".
func EvidenceSummary(r classify.Result) string {
	var parts []string
	for _, e := range r.Evidences {
		kind := strings.TrimPrefix(string(e.Kind), "cname_")
		kind = strings.TrimPrefix(kind, "ns_")
		if label := hostLabel(e.Host, r.Domain); label != "" {
			parts = append(parts, fmt.Sprintf("%s:%s→%s", kind, label, e.Record))
			continue
		}
		parts = append(parts, kind+":"+e.Record)
	}
	return strings.Join(parts, "; ")
}

// hostLabel names the queried host relative to the apex: "www" for the
// www host, "" for the apex itself, the full host otherwise.
func hostLabel(host, apex string) string {
	switch {
	case host == apex:
		return ""
	case strings.HasPrefix(host, "www."):
		return "www"
	default:
		return host
	}
}

func verdictCell(v classify.Verdict) string {
	if !v.Detected {
		return "-"
	}
	return string(v.Confidence)
}
