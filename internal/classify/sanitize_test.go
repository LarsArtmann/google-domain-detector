package classify

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestSanitizeEvidenceCoercesHostileStrings(t *testing.T) {
	t.Parallel()

	hostile := []Evidence{
		{
			Kind: EvidenceMXGoogle, Platform: PlatformWorkspace, Weight: WeightStrong,
			Host: "example.com", Record: "10 aspmx.l.google.com.",
		},
		{
			Kind: EvidenceSPFGoogle, Platform: PlatformWorkspace, Weight: WeightWeak,
			Host: "example.com", Record: "v=spf1 \xff\xfe include:_spf.google.com ~all",
		},
	}

	got := SanitizeEvidence(hostile)

	if len(got) != len(hostile) {
		t.Fatalf("len = %d, want %d", len(got), len(hostile))
	}

	if got[0].Record != hostile[0].Record {
		t.Errorf("clean record changed: %q", got[0].Record)
	}

	if !utf8.ValidString(got[1].Record) {
		t.Errorf("hostile record still invalid UTF-8: %q", got[1].Record)
	}

	if !strings.Contains(got[1].Record, "include:_spf.google.com") {
		t.Errorf("sanitization lost the SPF tail: %q", got[1].Record)
	}

	// The input slice must be untouched: other sinks may still hold it.
	if utf8.ValidString(hostile[1].Record) {
		t.Error("input evidence was mutated; SanitizeEvidence must copy")
	}

	if SanitizeEvidence(nil) != nil {
		t.Error("nil evidences should stay nil")
	}
}
