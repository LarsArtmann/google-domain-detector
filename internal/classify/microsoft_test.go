package classify

import (
	"net"
	"testing"
)

// microsoftEvidenceCount counts the microsoft-platform items in an evidence
// list — the assertion helper that keeps the table cases flat.
func microsoftEvidenceCount(evidences []Evidence) int {
	count := 0

	for _, e := range evidences {
		if e.Kind == EvidenceMXMicrosoft {
			count++
		}
	}

	return count
}

// TestClassifyMXMicrosoft pins the M365 MX fingerprint: tenant MX hosts are
// subdomains of mail.protection.outlook.com (the bare apex counts too, a
// label-boundary-safe equality like the Google rules use for SPF).
func TestClassifyMXMicrosoft(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		target string
		want   bool
	}{
		{"tenant subdomain", "acme-tenant.mail.protection.outlook.com.", true},
		{"bare fleet host", "mail.protection.outlook.com.", true},
		{"case-insensitive", "Mail.Protection.Outlook.COM.", true},
		{"label-boundary evil twin", "evil.mail.protection.outlook.com.evil.example.", false},
		{"outlook.com itself is not the fleet", "outlook.com.", false},
		{"google stays google", "smtp.google.com.", false},
		{"foreign", "mx.example.com.", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := ClassifyMX("acme.com", []*net.MX{{Host: tc.target}})

			if microsoftEvidenceCount(got) != boolCount(tc.want) {
				t.Fatalf("microsoft evidence count = %d, want %d: %+v",
					microsoftEvidenceCount(got), boolCount(tc.want), got)
			}

			if !tc.want {
				return
			}

			e := got[0]
			if e.Kind != EvidenceMXMicrosoft || e.Platform != PlatformMicrosoft || e.Weight != WeightStrong {
				t.Errorf("wrong evidence shape: %+v", e)
			}

			if e.Record != normalizeHost(tc.target) {
				t.Errorf("record should be the normalized target, got %q", e.Record)
			}
		})
	}
}

func boolCount(b bool) int {
	if b {
		return 1
	}

	return 0
}

// TestClassifyTXTMicrosoft pins the M365 SPF fingerprint and that Google
// detection is untouched by the shared parser.
func TestClassifyTXTMicrosoft(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		record string
		want   []EvidenceKind
	}{
		{
			"microsoft include",
			"v=spf1 include:spf.protection.outlook.com -all",
			[]EvidenceKind{EvidenceSPFMicrosoft},
		},
		{
			"both includes",
			"v=spf1 include:spf.protection.outlook.com include:_spf.google.com ~all",
			[]EvidenceKind{EvidenceSPFGoogle, EvidenceSPFMicrosoft},
		},
		{
			"microsoft include via subdomain",
			"v=spf1 include:x.spf.protection.outlook.com ~all",
			[]EvidenceKind{EvidenceSPFMicrosoft},
		},
		{
			"label-boundary evil twin",
			"v=spf1 include:spf.protection.outlook.com.evil.example ~all",
			nil,
		},
		{
			"not spf",
			"v=DMV1; verify",
			nil,
		},
		{
			"spf without microsoft",
			"v=spf1 include:_spf.google.com ~all",
			[]EvidenceKind{EvidenceSPFGoogle},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := ClassifyTXT("acme.com", []string{tc.record})

			if len(got) != len(tc.want) {
				t.Fatalf("expected %d evidence items, got %d: %+v", len(tc.want), len(got), got)
			}

			for i, wantKind := range tc.want {
				if got[i].Kind != wantKind {
					t.Errorf("evidence %d: want kind %s, got %s", i, wantKind, got[i].Kind)
				}
			}
		})
	}
}

// TestMicrosoftEvidenceNeverUpgradesVerdicts pins the platform boundary:
// Microsoft 365 evidence must leave every Google verdict untouched — a
// pure-M365 domain detects nothing, a mixed domain stays exactly as its
// Google evidence implies.
func TestMicrosoftEvidenceNeverUpgradesVerdicts(t *testing.T) {
	t.Parallel()

	pureM365 := Result{
		Domain: "acme.com",
		Evidences: []Evidence{{
			Kind: EvidenceMXMicrosoft, Platform: PlatformMicrosoft,
			Weight: WeightStrong, Host: "acme.com",
			Record: "acme.mail.protection.outlook.com",
		}},
	}

	if pureM365.WorkspaceVerdict().Detected || pureM365.GCPVerdict().Detected {
		t.Errorf("pure M365 domain must not produce any Google verdict")
	}

	googleMX := Evidence{
		Kind: EvidenceMXGoogle, Platform: PlatformWorkspace,
		Weight: WeightStrong, Host: "acme.com", Record: "smtp.google.com",
	}

	mixed := Result{Domain: "acme.com", Evidences: []Evidence{googleMX, pureM365.Evidences[0]}}
	mixedOnly := Result{Domain: "acme.com", Evidences: []Evidence{googleMX}}

	if mixed.WorkspaceVerdict() != mixedOnly.WorkspaceVerdict() {
		t.Errorf("microsoft evidence must not change a Google verdict: mixed %+v vs google-only %+v",
			mixed.WorkspaceVerdict(), mixedOnly.WorkspaceVerdict())
	}
}
