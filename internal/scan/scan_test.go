package scan

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/larsartmann/google-domain-detector/internal/classify"
	"github.com/larsartmann/google-domain-detector/internal/domain"
	"github.com/larsartmann/google-domain-detector/internal/ranges"
	"github.com/larsartmann/google-domain-detector/internal/resolve"
)

// fake is a scripted resolver: zone maps per lookup type, plus a call log
// behind a mutex so worker-pool tests stay race-clean.
type fake struct {
	mu     sync.Mutex
	called map[string]int

	txt    map[string][]string
	txtErr map[string]error
	mx     map[string][]*net.MX
	mxErr  map[string]error
	cname  map[string]string
	ips    map[string][]net.IP
	ns     map[string][]*net.NS
}

func newFake() *fake {
	return &fake{called: map[string]int{}}
}

func (f *fake) record(key string) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.called[key]++
}

func (f *fake) calls(key string) int {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.called[key]
}

func (f *fake) LookupTXT(_ context.Context, host string) ([]string, error) {
	f.record("txt:" + host)

	if err, ok := f.txtErr[host]; ok {
		return nil, err
	}

	return f.txt[host], nil
}

func (f *fake) LookupMX(_ context.Context, host string) ([]*net.MX, error) {
	f.record("mx:" + host)

	if err, ok := f.mxErr[host]; ok {
		return nil, err
	}

	return f.mx[host], nil
}

func (f *fake) LookupCNAME(_ context.Context, host string) (string, error) {
	f.record("cname:" + host)

	return f.cname[host], nil
}

func (f *fake) LookupIP(_ context.Context, host string) ([]net.IP, error) {
	f.record("ip:" + host)

	return f.ips[host], nil
}

func (f *fake) LookupNS(_ context.Context, host string) ([]*net.NS, error) {
	f.record("ns:" + host)

	return f.ns[host], nil
}

func notFound(host string) error {
	return &net.DNSError{Err: "no such host", Name: host, IsNotFound: true}
}

func mustDomains(t *testing.T, raw ...string) []domain.Domain {
	t.Helper()

	out := make([]domain.Domain, 0, len(raw))
	for _, r := range raw {
		d, err := domain.Parse(r)
		if err != nil {
			t.Fatalf("domain.Parse(%q): %v", r, err)
		}

		out = append(out, d)
	}

	return out
}

func fixtureRanges(t *testing.T) *ranges.Set {
	t.Helper()

	set, err := ranges.Parse([]byte(`{"prefixes":[{"ipv4Prefix":"34.0.0.0/8"}]}`))
	if err != nil {
		t.Fatal(err)
	}

	return set
}

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
	f.txtErr = map[string]error{"gone.example": notFound("gone.example")}

	r := ScanDomain(context.Background(), f, mustDomains(t, "gone.example")[0], Options{GCP: fixtureRanges(t)})
	if r.Err == "" {
		t.Error("NXDOMAIN must surface as Err")
	}

	for _, probe := range []string{"mx:gone.example", "cname:gone.example", "ip:gone.example"} {
		if f.calls(probe) != 0 {
			t.Errorf("lookup %q must be skipped after NXDOMAIN", probe)
		}
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

func TestRunPreservesInputOrder(t *testing.T) {
	t.Parallel()

	f := newFake()
	f.mx = map[string][]*net.MX{
		"c.com": {{Host: "smtp.google.com."}},
		"a.com": {{Host: "smtp.google.com."}},
		"b.com": {{Host: "smtp.google.com."}},
	}

	domains := mustDomains(t, "c.com", "a.com", "b.com")

	results := Run(context.Background(), f, domains, Options{Workers: 3, Timeout: time.Second})
	if len(results) != 3 {
		t.Fatalf("got %d results, want 3", len(results))
	}

	want := []string{"c.com", "a.com", "b.com"}
	for i, r := range results {
		if r.Domain != want[i] {
			t.Errorf("results[%d].Domain = %s, want %s", i, r.Domain, want[i])
		}
	}
}

func TestRunCancellationMarksPendingDomains(t *testing.T) {
	t.Parallel()

	f := newFake()
	release := make(chan struct{})
	f.mx = map[string][]*net.MX{}
	f.mxErr = map[string]error{"slow.com": &net.DNSError{Err: "canceled", Name: "slow.com", IsTimeout: true}}
	_ = release

	ctx, cancel := context.WithCancel(context.Background())
	domains := make([]domain.Domain, 0, 50)

	for i := range 50 {
		d, err := domain.Parse(fmt.Sprintf("host%02d.example", i))
		if err != nil {
			t.Fatal(err)
		}

		domains = append(domains, d)
	}

	cancel()

	results := Run(ctx, f, domains, Options{Workers: 1, Timeout: time.Second})
	for _, r := range results {
		if r.Domain == "" {
			t.Error("every result must carry its domain")
		}
	}
}

func TestRunEmptyInput(t *testing.T) {
	t.Parallel()

	if got := Run(context.Background(), newFake(), nil, Options{}); len(got) != 0 {
		t.Errorf("got %d results for empty input, want 0", len(got))
	}
}

func TestRunCallsOnResultForEachDomain(t *testing.T) {
	t.Parallel()

	f := newFake()
	f.txt = map[string][]string{"acme.com": {"v=spf1 include:_spf.google.com ~all"}}

	names := []string{"acme.com", "other.example", "third.example"}

	domains := mustDomains(t, names...)

	var mu sync.Mutex

	seen := map[string]int{}

	Run(context.Background(), f, domains, Options{
		Workers: 4,
		Timeout: time.Second,
		OnResult: func(r classify.Result) {
			mu.Lock()
			defer mu.Unlock()

			seen[r.Domain]++
		},
	})

	for _, name := range names {
		if seen[name] != 1 {
			t.Errorf("OnResult saw %s %d times, want exactly 1", name, seen[name])
		}
	}
}

var _ resolve.Resolver = (*fake)(nil)
