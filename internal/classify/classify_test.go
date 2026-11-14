package classify

import (
	"net"
	"testing"

	"github.com/larsartmann/google-domain-detector/internal/ranges"
)

func mx(hosts ...string) []*net.MX {
	out := make([]*net.MX, 0, len(hosts))
	for _, h := range hosts {
		out = append(out, &net.MX{Host: h, Pref: 1})
	}

	return out
}

func ns(hosts ...string) []*net.NS {
	out := make([]*net.NS, 0, len(hosts))
	for _, h := range hosts {
		out = append(out, &net.NS{Host: h})
	}

	return out
}

func kinds(es []Evidence) []EvidenceKind {
	out := make([]EvidenceKind, 0, len(es))
	for _, e := range es {
		out = append(out, e.Kind)
	}

	return out
}

func TestClassifyMX(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		hosts []string
		want  int
	}{
		"modern single": {[]string{"smtp.google.com."}, 1},
		"classic family": {
			[]string{"aspmx.l.google.com.", "alt1.aspmx.l.google.com.", "alt2.aspmx.l.google.com."},
			3,
		},
		"googlemail legacy": {[]string{"aspmx2.googlemail.com.", "aspmx3.googlemail.com."}, 2},
		"mixed google + microsoft": {
			[]string{"aspmx.l.google.com.", "mail.protection.outlook.com."},
			2,
		},
		"outlook only (microsoft evidence)": {[]string{"mail.protection.outlook.com."}, 1},
		"mimecast only":                     {[]string{"eu1.mimecast.com."}, 0},
		"googleLooksAlike":                  {[]string{"aspmx.l.google.com.evil.example."}, 0},
		"not google subdomain":              {[]string{"google.com."}, 0},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got := ClassifyMX("example.com", mx(tc.hosts...))
			if len(got) != tc.want {
				t.Fatalf("ClassifyMX(%v) = %v, want %d evidence items", tc.hosts, kinds(got), tc.want)
			}
		})
	}

	t.Run("googleLooksAlike produces nothing", func(t *testing.T) {
		t.Parallel()

		if got := ClassifyMX("example.com", mx("aspmx.l.google.com.evil.example.")); len(got) != 0 {
			t.Errorf("suffix attack must not match: %v", kinds(got))
		}
	})
}

func TestClassifyTXT(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		record string
		want   []EvidenceKind
	}{
		"plain include": {"v=spf1 include:_spf.google.com ~all", []EvidenceKind{EvidenceSPFGoogle}},
		"with microsoft includes": {
			"v=spf1 include:spf.protection.outlook.com include:_spf.google.com -all",
			[]EvidenceKind{EvidenceSPFGoogle, EvidenceSPFMicrosoft},
		},
		"uppercase version": {"V=SPF1 include:_spf.google.com ALL", []EvidenceKind{EvidenceSPFGoogle}},
		"wrong spf version": {"v=spf10 include:_spf.google.com ~all", nil},
		"include is-prefix": {"v=spf1 include:_spf.google.com.evil.example ~all", nil},
		"microsoft include": {"v=spf1 include:spf.protection.outlook.com ~all", []EvidenceKind{EvidenceSPFMicrosoft}},
		"verification":      {"google-site-verification=abc123", []EvidenceKind{EvidenceSiteVerification}},
		"dkim txt":          {"v=DKIM1; k=rsa; p=MIIB", nil},
		"google domainkey":  {"v=DKIM1; k=rsa; p=abc", nil},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got := ClassifyTXT("example.com", []string{tc.record})
			if len(got) != len(tc.want) {
				t.Fatalf("ClassifyTXT(%q) = %v, want %v", tc.record, kinds(got), tc.want)
			}

			for i := range got {
				if got[i].Kind != tc.want[i] {
					t.Errorf("evidence[%d].Kind = %s, want %s", i, got[i].Kind, tc.want[i])
				}
			}
		})
	}

	t.Run("verification and spf in separate records", func(t *testing.T) {
		t.Parallel()

		got := ClassifyTXT("example.com", []string{
			"google-site-verification=abc",
			"v=spf1 include:_spf.google.com ~all",
		})
		if len(got) != 2 {
			t.Errorf("want 2 evidence items, got %v", kinds(got))
		}
	})
}

func TestClassifyCNAME(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		target string
		want   EvidenceKind
	}{
		"cloud run":        {"foo-bar-uc.a.run.app.", EvidenceCNAMERunApp},
		"app engine":       {"sharp-service.ew.r.appspot.com.", EvidenceCNAMEAppspot},
		"firebase web.app": {"my-site.web.app.", EvidenceCNAMEFirebase},
		"firebaseapp":      {"my-site.firebaseapp.com.", EvidenceCNAMEFirebase},
		"functions":        {"region-project.cloudfunctions.net.", EvidenceCNAMEFunctions},
		"googlehosted":     {"ghs.googlehosted.com.", EvidenceCNAMEGoogleHosted},
		"gcs website":      {"my-bucket.storage.googleapis.com.", EvidenceCNAMEGCS},
		"gcs c endpoint":   {"c.storage.googleapis.com.", EvidenceCNAMEGCS},
		"self":             {"www.example.com.", ""},
		"foreign":          {"shops.myshopify.com.", ""},
		"suffix attack":    {"foo.web.app.evil.example.", ""},
		"cloudflare":       {"xxx.cf.dsrs.dev.", ""},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got := ClassifyCNAME("www.example.com", tc.target)
			if tc.want == "" {
				if len(got) != 0 {
					t.Fatalf("ClassifyCNAME(%q) = %v, want none", tc.target, kinds(got))
				}

				return
			}

			if len(got) != 1 || got[0].Kind != tc.want {
				t.Fatalf("ClassifyCNAME(%q) = %v, want [%s]", tc.target, kinds(got), tc.want)
			}

			if got[0].Platform != PlatformGCP || got[0].Weight != WeightStrong {
				t.Errorf("evidence must be strong GCP, got %+v", got[0])
			}
		})
	}
}

func TestClassifyNS(t *testing.T) {
	t.Parallel()

	if got := ClassifyNS(
		"example.com",
		ns("ns-cloud-a1.googledomains.com.", "ns-cloud-a2.googledomains.com."),
	); len(
		got,
	) != 2 {
		t.Errorf("want 2 Cloud DNS evidence items, got %v", kinds(got))
	}

	if got := ClassifyNS("example.com", ns("ns1.example.com.")); len(got) != 0 {
		t.Errorf("self-hosted NS must not match: %v", kinds(got))
	}

	if got := ClassifyNS("example.com", ns("aws1.cloudns.net.")); len(got) != 0 {
		t.Errorf("foreign NS must not match: %v", kinds(got))
	}
}

func TestClassifyIPs(t *testing.T) {
	t.Parallel()

	set := mustParseRanges(t, `{"prefixes":[{"ipv4Prefix":"34.0.0.0/8"},{"ipv6Prefix":"2600:1900::/28"}]}`)
	tests := map[string]struct {
		ip   string
		want bool
	}{
		"gcp v4":        {"34.64.0.1", true},
		"gcp v6":        {"2600:1900:1:2::1", true},
		"cloudflare v4": {"104.16.0.1", false},
		"aws v4":        {"52.1.2.3", false},
		"google corp":   {"142.250.0.1", false}, // goog.json space, not cloud.json
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got := ClassifyIPs("example.com", []net.IP{net.ParseIP(tc.ip)}, set)
			if tc.want && len(got) != 1 {
				t.Fatalf("ip %s should match, got %v", tc.ip, kinds(got))
			}

			if !tc.want && len(got) != 0 {
				t.Fatalf("ip %s should not match, got %v", tc.ip, kinds(got))
			}
		})
	}

	if got := ClassifyIPs("example.com", []net.IP{net.ParseIP("34.1.1.1")}, nil); len(got) != 0 {
		t.Errorf("nil set must disable matching, got %v", kinds(got))
	}
}

func TestWorkspaceVerdict(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		evidence []Evidence
		want     Verdict
	}{
		"mx certain": {
			[]Evidence{{Kind: EvidenceMXGoogle, Platform: PlatformWorkspace, Weight: WeightStrong}},
			Verdict{true, ConfidenceCertain},
		},
		"spf likely": {
			[]Evidence{{Kind: EvidenceSPFGoogle, Platform: PlatformWorkspace, Weight: WeightWeak}},
			Verdict{true, ConfidenceLikely},
		},
		"dkim likely": {
			[]Evidence{{Kind: EvidenceDKIMGoogle, Platform: PlatformWorkspace, Weight: WeightWeak}},
			Verdict{true, ConfidenceLikely},
		},
		"mx beats spf": {
			[]Evidence{
				{Kind: EvidenceSPFGoogle, Platform: PlatformWorkspace, Weight: WeightWeak},
				{Kind: EvidenceMXGoogle, Platform: PlatformWorkspace, Weight: WeightStrong},
			},
			Verdict{true, ConfidenceCertain},
		},
		"gcp is not workspace": {
			[]Evidence{{Kind: EvidenceCNAMERunApp, Platform: PlatformGCP, Weight: WeightStrong}},
			Verdict{false, ConfidenceNone},
		},
		"verification only": {
			[]Evidence{{Kind: EvidenceSiteVerification, Platform: PlatformGoogle, Weight: WeightWeak}},
			Verdict{false, ConfidenceNone},
		},
		"nothing": {nil, Verdict{false, ConfidenceNone}},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			r := Result{Domain: "example.com", Evidences: tc.evidence}

			got := r.WorkspaceVerdict()
			if got != tc.want {
				t.Errorf("WorkspaceVerdict() = %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestGCPVerdict(t *testing.T) {
	t.Parallel()

	strong := []Evidence{{Kind: EvidenceIPGCPRange, Platform: PlatformGCP, Weight: WeightStrong}}

	r := Result{Domain: "example.com", Evidences: strong}
	if r.GCPVerdict() != (Verdict{true, ConfidenceCertain}) {
		t.Errorf("strong GCP evidence must be certain, got %+v", r.GCPVerdict())
	}

	none := Result{Domain: "example.com", Evidences: []Evidence{
		{Kind: EvidenceMXGoogle, Platform: PlatformWorkspace, Weight: WeightStrong},
	}}
	if none.GCPVerdict().Detected {
		t.Error("workspace evidence must not imply GCP")
	}
}

func mustParseRanges(t *testing.T, data string) *ranges.Set {
	t.Helper()

	set, err := ranges.Parse([]byte(data))
	if err != nil {
		t.Fatalf("parse range fixture: %v", err)
	}

	return set
}

func TestCountTalliesVerdicts(t *testing.T) {
	t.Parallel()

	results := []Result{
		{Domain: "certain.example", Evidences: []Evidence{
			{Platform: PlatformWorkspace, Weight: WeightStrong},
		}},
		{Domain: "likely.example", Evidences: []Evidence{
			{Platform: PlatformWorkspace, Weight: WeightWeak},
		}},
		{Domain: "gcp.example", Evidences: []Evidence{
			{Platform: PlatformGCP, Weight: WeightStrong},
			{Platform: PlatformGoogle, Weight: WeightWeak},
		}},
		{Domain: "gone.example", Err: "domain not found", ErrKind: ErrorNXDomain},
	}

	got := Count(results)

	want := Counts{
		Scanned:          4,
		WorkspaceCertain: 1,
		WorkspaceLikely:  1,
		GCPCertain:       1,
		GenericGoogle:    1,
		Errors:           1,
		ErrorKinds:       ErrorKinds{NXDomain: 1},
	}

	if got != want {
		t.Errorf("Count = %+v, want %+v", got, want)
	}
}
