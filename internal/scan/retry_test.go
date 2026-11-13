package scan

import (
	"context"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/larsartmann/google-domain-detector/internal/classify"
	"github.com/larsartmann/google-domain-detector/internal/resolve"
)

// flakyResolver fails the first TXT lookups with a transient error, then
// delegates to the scripted inner resolver.
type flakyResolver struct {
	inner     resolve.Resolver
	failFirst atomic.Int32
}

func timeoutErr(host string) error {
	return &net.DNSError{Err: "i/o timeout", Name: host, IsTimeout: true, IsTemporary: true}
}

func (f *flakyResolver) LookupTXT(ctx context.Context, host string) ([]string, error) {
	if f.failFirst.Add(-1) >= 0 {
		return nil, timeoutErr(host)
	}

	return f.inner.LookupTXT(ctx, host)
}

func (f *flakyResolver) LookupMX(ctx context.Context, host string) ([]*net.MX, error) {
	return f.inner.LookupMX(ctx, host)
}

func (f *flakyResolver) LookupCNAME(ctx context.Context, host string) (string, error) {
	return f.inner.LookupCNAME(ctx, host)
}

func (f *flakyResolver) LookupIP(ctx context.Context, host string) ([]net.IP, error) {
	return f.inner.LookupIP(ctx, host)
}

func (f *flakyResolver) LookupNS(ctx context.Context, host string) ([]*net.NS, error) {
	return f.inner.LookupNS(ctx, host)
}

func (f *flakyResolver) LookupExistence(ctx context.Context, host string) (resolve.Existence, error) {
	return f.inner.LookupExistence(ctx, host)
}

func TestTransientFailureIsRetriedOnce(t *testing.T) {
	t.Parallel()

	inner := newFake()
	inner.txt = map[string][]string{"acme.com": {"v=spf1 include:_spf.google.com ~all"}}

	flaky := &flakyResolver{inner: inner}
	flaky.failFirst.Store(1)

	results := Run(context.Background(), flaky, mustDomains(t, "acme.com"), Options{Timeout: time.Second})

	if results[0].Err != "" {
		t.Fatalf("one transient failure must be cleared by the retry, got %q", results[0].Err)
	}

	if !results[0].WorkspaceVerdict().Detected {
		t.Error("retried lookup must recover the SPF evidence")
	}
}

func TestPersistentTransientFailureRetriesExactlyOnce(t *testing.T) {
	t.Parallel()

	f := newFake()
	f.txtErr = map[string]error{"slow.example": timeoutErr("slow.example")}

	results := Run(context.Background(), f, mustDomains(t, "slow.example"), Options{Timeout: time.Second})

	if results[0].ErrKind != classify.ErrorTimeout {
		t.Errorf("ErrKind = %q, want timeout", results[0].ErrKind)
	}

	if calls := f.calls("txt:slow.example"); calls != 2 {
		t.Errorf("persistent transient failure must see exactly 2 attempts (1 + retry), got %d", calls)
	}
}

func TestNXDOMAINIsNotRetried(t *testing.T) {
	t.Parallel()

	f := newFake()
	f.txtErr = map[string]error{"gone.example": notFound("gone.example")}

	results := Run(context.Background(), f, mustDomains(t, "gone.example"), Options{Timeout: time.Second})

	if results[0].ErrKind != classify.ErrorNXDomain {
		t.Errorf("ErrKind = %q, want nxdomain", results[0].ErrKind)
	}

	if calls := f.calls("txt:gone.example"); calls != 1 {
		t.Errorf("an authoritative NXDOMAIN must never be retried, got %d calls", calls)
	}
}
