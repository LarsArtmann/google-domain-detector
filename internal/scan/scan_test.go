package scan

import (
	"context"
	"net"
	"sync"
	"testing"

	"github.com/larsartmann/google-domain-detector/internal/domain"
	"github.com/larsartmann/google-domain-detector/internal/ranges"
	"github.com/larsartmann/google-domain-detector/internal/resolve"
)

// fake is a scripted resolver: zone maps per lookup type, plus a call log
// behind a mutex so worker-pool tests stay race-clean.
type fake struct {
	mu     sync.Mutex
	called map[string]int

	onTXT  func(ctx context.Context, host string)
	txt    map[string][]string
	txtErr map[string]error
	mx     map[string][]*net.MX
	mxErr  map[string]error
	cname  map[string]string
	ips    map[string][]net.IP
	ns     map[string][]*net.NS
	// existence defaults to Exists per host; overrides script dead names
	// and oracle trouble.
	existence    map[string]resolve.Existence
	existenceErr map[string]error
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

func (f *fake) LookupTXT(ctx context.Context, host string) ([]string, error) {
	f.record("txt:" + host)

	if f.onTXT != nil {
		f.onTXT(ctx, host)
	}

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

func (f *fake) LookupExistence(_ context.Context, host string) (resolve.Existence, error) {
	f.record("exists:" + host)

	if err, ok := f.existenceErr[host]; ok {
		return resolve.Exists, err
	}

	if existence, ok := f.existence[host]; ok {
		return existence, nil
	}

	return resolve.Exists, nil
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
