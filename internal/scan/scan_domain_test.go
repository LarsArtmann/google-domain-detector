package scan

import (
	"context"
	"errors"
	"net"
	"testing"

	"github.com/larsartmann/google-domain-detector/internal/classify"
	"github.com/larsartmann/google-domain-detector/internal/resolve"
)

func TestScanDomainWorkspaceViaMX(t *testing.T) {
	t.Parallel()

	f := newFake()
	f.txt = map[string][]string{"acme.com": {"google-site-verification=abc"}}
	f.mx = map[string][]*net.MX{"acme.com": {{Host: "smtp.google.com."}}}

	r := ScanDomain(context.Background(), f, mustDomains(t, "acme.com")[0], Options{ProbeDKIM: true})
	if v := r.WorkspaceVerdict(); !v.Detected || v.Confidence != "certain" {
		t.Errorf("workspace verdict = %+v, want certain", v)
	}

	if f.calls("txt:google._domainkey.acme.com") != 0 {
		t.Error("DKIM probe must be skipped once MX proves Workspace")
	}
}

func TestScanDomainWorkspaceLikelyViaDKIMAlone(t *testing.T) {
	t.Parallel()

	// SPF and MX are silent (Outlook-fronted mail, say); the DKIM probe
	// runs and its presence alone yields the likely verdict.
	f := newFake()
	f.txt = map[string][]string{
		"acme.com":                   {"v=spf1 include:spf.protection.outlook.com ~all"},
		"google._domainkey.acme.com": {"v=DKIM1; p=x"},
	}

	r := ScanDomain(context.Background(), f, mustDomains(t, "acme.com")[0], Options{ProbeDKIM: true})
	if v := r.WorkspaceVerdict(); !v.Detected || v.Confidence != "likely" {
		t.Errorf("workspace verdict = %+v, want likely", v)
	}

	if f.calls("txt:google._domainkey.acme.com") != 1 {
		t.Error("DKIM probe must run when MX and SPF are silent")
	}
}

func TestScanDomainGCPCNAMEAtWWW(t *testing.T) {
	t.Parallel()

	f := newFake()
	f.cname = map[string]string{
		"acme.com":     "acme.com", // apex: no CNAME, canonical itself
		"www.acme.com": "acme-website.web.app.",
	}

	r := ScanDomain(context.Background(), f, mustDomains(t, "acme.com")[0], Options{})
	if v := r.GCPVerdict(); !v.Detected {
		t.Errorf("gcp verdict = %+v, want detected", v)
	}

	if v := r.WorkspaceVerdict(); v.Detected {
		t.Error("hosting evidence must not imply Workspace")
	}
}

func TestScanDomainGCPViaIPRange(t *testing.T) {
	t.Parallel()

	f := newFake()
	f.ips = map[string][]net.IP{
		"acme.com":     {net.ParseIP("104.16.1.1")},
		"www.acme.com": {net.ParseIP("34.120.0.5")},
	}

	r := ScanDomain(context.Background(), f, mustDomains(t, "acme.com")[0], Options{GCP: fixtureRanges(t)})
	if !r.GCPVerdict().Detected {
		t.Error("www IP inside 34/8 must be GCP evidence")
	}
}

func TestScanDomainCloudDNS(t *testing.T) {
	t.Parallel()

	f := newFake()
	f.ns = map[string][]*net.NS{"acme.com": {{Host: "ns-cloud-a1.googledomains.com."}}}

	r := ScanDomain(context.Background(), f, mustDomains(t, "acme.com")[0], Options{})
	if !r.GCPVerdict().Detected {
		t.Error("Cloud DNS nameservers must be GCP evidence")
	}
}

func TestScanDomainNotFoundShortCircuits(t *testing.T) {
	t.Parallel()

	f := newFake()
	f.existence = map[string]resolve.Existence{"gone.example": resolve.NotExists}

	r := ScanDomain(context.Background(), f, mustDomains(t, "gone.example")[0], Options{GCP: fixtureRanges(t)})
	if r.Err == "" {
		t.Error("NXDOMAIN must surface as Err")
	}

	for _, probe := range []string{"txt:gone.example", "mx:gone.example", "cname:gone.example", "ip:gone.example"} {
		if f.calls(probe) != 0 {
			t.Errorf("lookup %q must be skipped after NXDOMAIN", probe)
		}
	}
}

// TestScanDomainNoMXIsNotNX pins the 2026-11-13 census bug: a live name
// (oracle says Exists) whose MX lookup answers NODATA — Go maps that to
// the same IsNotFound error as NXDOMAIN — must surface clean, not as a
// domain-truth NXDOMAIN verdict. kwai.net was the live specimen.
func TestScanDomainNoMXIsNotNX(t *testing.T) {
	t.Parallel()

	f := newFake()
	f.txt = map[string][]string{"kwai.example": {"fastly-domain-delegation-x"}}
	f.mxErr = map[string]error{"kwai.example": notFound("kwai.example")}

	r := ScanDomain(context.Background(), f, mustDomains(t, "kwai.example")[0], Options{GCP: fixtureRanges(t)})
	if r.Err != "" {
		t.Errorf("NODATA on MX of an existing name must not set Err, got %q (%v)", r.Err, r.ErrKind)
	}

	if r.ErrKind != classify.ErrorNone {
		t.Errorf("ErrKind = %q, want none", r.ErrKind)
	}
}

func TestScanDomainNonFatalLookupError(t *testing.T) {
	t.Parallel()

	f := newFake()
	f.mxErr = map[string]error{"acme.com": errors.New("temporary failure")}
	f.txt = map[string][]string{"acme.com": {"google-site-verification=abc"}}

	r := ScanDomain(context.Background(), f, mustDomains(t, "acme.com")[0], Options{})
	if r.Err != "" {
		t.Errorf("a failed lookup alongside evidence must not set Err, got %q", r.Err)
	}
}
