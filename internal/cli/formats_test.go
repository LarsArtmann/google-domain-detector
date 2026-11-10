package cli

import (
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/larsartmann/google-domain-detector/internal/render"
)

// TestFormatEnumsMatchRegistry pins every --format flag's values tag to
// render.SupportedFormats(), which is derived from go-output's renderer
// registries at runtime. Drift in either direction fails the build: a
// blank import dropped (a format silently disappears) or a renderer added
// (a flag rejects a working format) must be reconciled with the tags.
func TestFormatEnumsMatchRegistry(t *testing.T) {
	t.Parallel()

	want := render.SupportedFormats()
	wantTokens := make([]string, len(want))
	for i, f := range want {
		wantTokens[i] = string(f)
	}

	slices.Sort(wantTokens)

	wantJoined := strings.Join(wantTokens, ",")

	for _, tc := range []struct {
		typeName string
		tag      string
	}{
		{"ScanFlags", formatValuesTag[ScanFlags]()},
		{"ReplayFlags", formatValuesTag[ReplayFlags]()},
		{"ShowFlags", formatValuesTag[ShowFlags]()},
	} {
		if tc.tag == "" {
			t.Fatalf("%s has no --format values tag", tc.typeName)
		}

		gotTokens := strings.Split(tc.tag, ",")
		slices.Sort(gotTokens)

		if got := strings.Join(gotTokens, ","); got != wantJoined {
			t.Errorf("%s --format enum %q drifted from the registry %q — update the tag or the renderer set", tc.typeName, tc.tag, wantJoined)
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
