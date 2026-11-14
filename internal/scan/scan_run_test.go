package scan

import (
	"context"
	"fmt"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/larsartmann/google-domain-detector/internal/classify"
	"github.com/larsartmann/google-domain-detector/internal/domain"
	"github.com/larsartmann/google-domain-detector/internal/resolve"
)

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

	res := newFake()
	release := make(chan struct{})
	res.mx = map[string][]*net.MX{}
	res.mxErr = map[string]error{"slow.com": &net.DNSError{Err: "canceled", Name: "slow.com", IsTimeout: true}}
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

	results := Run(ctx, res, domains, Options{Workers: 1, Timeout: time.Second})
	for i, r := range results {
		if r.Domain == "" {
			t.Error("every result must carry its domain")
		}

		if !r.Unscanned {
			t.Errorf("result %d (%s) was never scanned; it must be marked Unscanned so counts exclude it", i, r.Domain)
		}
	}

	if counts := classify.Count(results); counts.Scanned != 0 || counts.Errors != 0 {
		t.Errorf("unscanned placeholders must tally nowhere, got %+v", counts)
	}
}

// TestRunMarksOnlyNeverReachedDomainsUnscanned separates the two cancel
// classes: a domain whose lookup was in flight when the run died is a real
// scan (canceled, journaled), while a domain the pool never reached is an
// Unscanned placeholder (excluded from counts and the journal).
func TestRunMarksOnlyNeverReachedDomainsUnscanned(t *testing.T) {
	t.Parallel()

	res := newFake()
	res.txtErr = map[string]error{"slow.com": context.Canceled}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	started := make(chan struct{})

	slow, err := domain.Parse("slow.com")
	if err != nil {
		t.Fatal(err)
	}

	never, err := domain.Parse("never.example")
	if err != nil {
		t.Fatal(err)
	}

	// One worker: it enters slow.com's TXT lookup, signals, and holds there
	// until the run dies (plus a grace period, so the feeder's parked
	// select reliably takes ctx.Done) — never.example can never be fed.
	var once sync.Once

	res.onTXT = func(ctx context.Context, host string) {
		if host != "slow.com" {
			return
		}

		once.Do(func() {
			close(started)
			<-ctx.Done()
			time.Sleep(100 * time.Millisecond)
		})
	}

	go func() {
		<-started
		cancel()
	}()

	results := Run(ctx, res, []domain.Domain{slow, never}, Options{Workers: 1, Timeout: 10 * time.Second})

	if results[0].Unscanned {
		t.Error("slow.com was in flight when canceled: a real scan, not a placeholder")
	}

	if !results[1].Unscanned {
		t.Error("never.example was never fed to the pool: it must be marked Unscanned")
	}

	if counts := classify.Count(results); counts.Scanned != 1 {
		t.Errorf("only the in-flight domain counts as scanned, got %+v", counts)
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

func TestRunCallsOnDomainStartForEachDomain(t *testing.T) {
	t.Parallel()

	f := newFake()
	f.txt = map[string][]string{"acme.com": {"v=spf1 include:_spf.google.com ~all"}}

	names := []string{"acme.com", "other.example"}

	var mu sync.Mutex

	started := map[string]int{}

	Run(context.Background(), f, mustDomains(t, names...), Options{
		Workers: 4,
		Timeout: time.Second,
		OnDomainStart: func(domain string) {
			mu.Lock()
			defer mu.Unlock()

			started[domain]++
		},
	})

	for _, name := range names {
		if started[name] != 1 {
			t.Errorf("OnDomainStart saw %s %d times, want exactly 1", name, started[name])
		}
	}
}

var _ resolve.Resolver = (*fake)(nil)
