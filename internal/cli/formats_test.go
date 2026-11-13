package cli

import (
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/larsartmann/google-domain-detector/internal/render"
)

// formatContract is the product decision for which formats --format offers:
// table-shaped reports via render.Table (table, csv, tsv, markdown, yaml,
// toml) plus the full-evidence DTO renderers (json, jsonl).
var formatContract = []string{"csv", "json", "jsonl", "markdown", "table", "toml", "tsv", "yaml"}

// TestFormatEnumsMatchRegistry guards the --format enum in both directions:
// every contract format must exist in go-output's renderer registries (a
// dropped blank import would silently remove a format — fail the build),
// and every flag's values tag must carry exactly the contract (product
// drift is caught, not absorbed). Formats registered by other in-process
// consumers of go-output (cmdguard's tree/markup batteries) are bystanders:
// their presence in the registry neither requires nor forbids a flag entry.
func TestFormatEnumsMatchRegistry(t *testing.T) {
	t.Parallel()

	registry := map[string]bool{}
	for _, f := range render.SupportedFormats() {
		registry[string(f)] = true
	}

	want := slices.Clone(formatContract)
	slices.Sort(want)
	wantJoined := strings.Join(want, ",")

	for _, token := range formatContract {
		if !registry[token] {
			t.Errorf(
				"format %q is in the --format contract but no renderer is registered — a blank import was likely dropped",
				token,
			)
		}
	}

	for _, tc := range []struct {
		typeName string
		tag      string
	}{
		{"ScanFlags", formatValuesTag[ScanFlags]()},
		{"ReplayFlags", formatValuesTag[ReplayFlags]()},
		{"ShowFlags", formatValuesTag[ShowFlags]()},
		{"RunsFlags", formatValuesTag[RunsFlags]()},
	} {
		if tc.tag == "" {
			t.Fatalf("%s has no --format values tag", tc.typeName)
		}

		got := slices.Clone(strings.Split(tc.tag, ","))
		slices.Sort(got)

		if joined := strings.Join(got, ","); joined != wantJoined {
			t.Errorf(
				"%s --format enum %q drifted from the contract %q — update the tag or the contract",
				tc.typeName,
				tc.tag,
				wantJoined,
			)
		}
	}
}

func formatValuesTag[T any]() string {
	field, found := reflect.TypeFor[T]().FieldByName("Format")
	if !found {
		return ""
	}

	return field.Tag.Get("values")
}
