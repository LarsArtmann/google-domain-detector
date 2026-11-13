package scan

import (
	"context"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/larsartmann/go-codec"

	"github.com/larsartmann/google-domain-detector/internal/domain"
)

// A hostile TXT record carried raw invalid UTF-8 into a wave-B result, whose
// JSON v2 encoder then refused the line and wedged the whole render path
// (2026-11-13, tranco1m-b). ScanDomain must coerce DNS-derived strings to
// valid UTF-8 before anything downstream sees them.
func TestScanDomainSanitizesHostileRecordBytes(t *testing.T) {
	t.Parallel()

	hostile := "google-site-verification=\xff\xfe\x80 raw bytes"
	f := newFake()
	f.txt = map[string][]string{"hostile.com": {hostile}}

	dom, err := domain.Parse("hostile.com")
	if err != nil {
		t.Fatalf("domain.Parse: %v", err)
	}

	got := ScanDomain(context.Background(), f, dom, Options{})

	if got.Domain != "hostile.com" {
		t.Fatalf("domain = %q", got.Domain)
	}

	if len(got.Evidences) == 0 {
		t.Fatal("no evidence collected from the hostile TXT")
	}

	for _, ev := range got.Evidences {
		if !utf8.ValidString(ev.Record) {
			t.Errorf("evidence record still invalid UTF-8: %q", ev.Record)
		}
	}

	if !utf8.ValidString(got.Err) {
		t.Errorf("result error still invalid UTF-8: %q", got.Err)
	}

	// The regression: encoding/json v2 (jsontext) refuses invalid UTF-8 in
	// strings. The sanitized result must encode cleanly.
	var raw []byte

	raw, encErr := (codec.JSONCodec{}).Encode(got)
	if encErr != nil {
		t.Fatalf("json encode of scanned result: %v", encErr)
	}

	if !strings.Contains(string(raw), "google-site-verification=") {
		t.Errorf("sanitization lost the record prefix: %s", raw)
	}
}
