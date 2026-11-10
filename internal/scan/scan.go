// Package scan orchestrates per-domain DNS probing behind a bounded worker
// pool. Input order is preserved; every domain gets an independent timeout.
package scan

import (
	"context"
	"errors"
	"net"
	"sync"
	"time"

	"github.com/larsartmann/google-domain-detector/internal/classify"
	"github.com/larsartmann/google-domain-detector/internal/domain"
	"github.com/larsartmann/google-domain-detector/internal/ranges"
	"github.com/larsartmann/google-domain-detector/internal/resolve"
)

// defaultWorkers is the concurrency used when Options.Workers is unset.
const defaultWorkers = 8

// Options configures one scan run.
type Options struct {
	// Workers bounds the parallel lookups; <= 0 means defaultWorkers.
	Workers int
	// Timeout bounds all lookups for a single domain.
	Timeout time.Duration
	// GCP enables IP-range matching when non-nil; nil skips IP evidence.
	GCP *ranges.Set
	// ProbeDKIM queries google._domainkey.<domain> for domains whose MX and
	// SPF lookups yielded no Workspace evidence.
	ProbeDKIM bool
	// OnResult, when set, is invoked with every completed result, possibly
	// concurrently from worker goroutines. It must not block unduly: slow
	// sinks stall the whole pool.
	OnResult func(classify.Result)
	// OnDomainStart, when set, is invoked once per domain as its lookup
	// begins (before the result lands), possibly concurrently. It must not
	// block unduly: slow callbacks stall the whole pool. Progress UIs use
	// it to show domains as running before they finish.
	OnDomainStart func(domain string)
}

// Run scans every domain with a bounded worker pool and returns results in
// input order. Context cancellation aborts pending domains; their results
// carry the cancellation as Err.
func Run(ctx context.Context, res resolve.Resolver, domains []domain.Domain, opts Options) []classify.Result {
	results := make([]classify.Result, len(domains))
	if len(domains) == 0 {
		return results
	}

	// One retry per lookup clears transient resolver trouble without hiding
	// authoritative answers (NXDOMAIN is never retried).
	res = retryResolver{inner: res}

	workers := resolveWorkers(opts.Workers, len(domains))

	queue := make(chan int)

	var wg sync.WaitGroup
	for range workers {
		wg.Go(func() {
			for i := range queue {
				if opts.OnDomainStart != nil {
					opts.OnDomainStart(domains[i].String())
				}

				dctx, cancel := context.WithTimeout(ctx, opts.Timeout)
				results[i] = ScanDomain(dctx, res, domains[i], opts)

				if opts.OnResult != nil {
					opts.OnResult(results[i])
				}

				cancel()
			}
		})
	}

	feed := func(i int) bool {
		select {
		case queue <- i:
			return true
		case <-ctx.Done():
			return false
		}
	}
	for i := range domains {
		if !feed(i) {
			break
		}
	}

	close(queue)
	wg.Wait()

	fillUnrunResults(ctx, results, domains)

	return results
}

// resolveWorkers clamps the requested parallelism to the pool defaults and
// the input size.
func resolveWorkers(requested, domainCount int) int {
	workers := requested
	if workers <= 0 {
		workers = defaultWorkers
	}

	if workers > domainCount {
		workers = domainCount
	}

	return workers
}

// fillUnrunResults turns slots a canceled run never reached into explicit
// cancellation results, so the report accounts for every input domain.
func fillUnrunResults(ctx context.Context, results []classify.Result, domains []domain.Domain) {
	for i := range results {
		if results[i].Domain == "" {
			err := ctx.Err()
			results[i] = classify.Result{
				Domain:  domains[i].String(),
				Err:     err.Error(),
				ErrKind: classify.ErrorKindOf(err),
			}
		}
	}
}

// ScanDomain probes one domain: TXT (SPF, site verification) and MX at the
// apex, NS at the apex, CNAME and IP-range evidence at apex and www. The DKIM
// selector is only queried when MX and SPF were both silent, so proven
// Workspace domains cost no extra lookup. NXDOMAIN on the first lookup
// short-circuits the domain.
func ScanDomain(ctx context.Context, res resolve.Resolver, d domain.Domain, opts Options) classify.Result {
	apex := d.String()
	result := classify.Result{Domain: apex}

	lookupErr := probeApexRecords(ctx, res, apex, opts, &result)
	if lookupErr == errShortCircuit {
		return result
	}

	lookupErr = firstErr(lookupErr, probeHosts(ctx, res, d, opts, &result))

	if lookupErr != nil && len(result.Evidences) == 0 {
		result.Err = lookupErr.Error()
		result.ErrKind = classify.ErrorKindOf(lookupErr)
	}

	return result
}

// errShortCircuit signals that the domain answered NXDOMAIN: no further
// lookups can change the verdict.
var errShortCircuit = errors.New("nxdomain short-circuit")

// probeApexRecords collects apex TXT/SPF, MX, DKIM (only when MX+SPF are
// silent), and NS evidence. The returned error is the first non-NXDOMAIN
// lookup failure, or errShortCircuit when the domain does not exist.
func probeApexRecords(ctx context.Context, res resolve.Resolver, apex string, opts Options, result *classify.Result) error {
	var lookupErr error

	txts, err := res.LookupTXT(ctx, apex)
	if err != nil {
		if dnsErr, ok := errors.AsType[*net.DNSError](err); ok && dnsErr.IsNotFound {
			result.Err = "domain not found: " + err.Error()
			result.ErrKind = classify.ErrorNXDomain

			return errShortCircuit
		}

		lookupErr = err
	} else {
		result.Evidences = append(result.Evidences, classify.ClassifyTXT(apex, txts)...)
	}

	if mxs, err := res.LookupMX(ctx, apex); err != nil {
		lookupErr = firstErr(lookupErr, err)
	} else {
		result.Evidences = append(result.Evidences, classify.ClassifyMX(apex, mxs)...)
	}

	if opts.ProbeDKIM && !result.WorkspaceVerdict().Detected {
		if dkim, err := res.LookupTXT(ctx, "google._domainkey."+apex); err == nil {
			result.Evidences = append(result.Evidences, classify.ClassifyDKIM(apex, dkim)...)
		}
		// A missing selector is the normal case, not an error worth reporting.
	}

	if nss, err := res.LookupNS(ctx, apex); err != nil {
		lookupErr = firstErr(lookupErr, err)
	} else {
		result.Evidences = append(result.Evidences, classify.ClassifyNS(apex, nss)...)
	}

	return lookupErr
}

// probeHosts collects CNAME and IP-range evidence for every host of the
// domain (apex, then www). CNAME absence is normal for A-record hosts and
// never an error; the returned error is the first IP lookup failure.
func probeHosts(ctx context.Context, res resolve.Resolver, d domain.Domain, opts Options, result *classify.Result) error {
	var lookupErr error

	for _, host := range d.Hosts() {
		if target, err := res.LookupCNAME(ctx, host); err == nil {
			result.Evidences = append(result.Evidences, classify.ClassifyCNAME(host, target)...)
		}

		if opts.GCP != nil {
			if ips, err := res.LookupIP(ctx, host); err != nil {
				lookupErr = firstErr(lookupErr, err)
			} else {
				result.Evidences = append(result.Evidences, classify.ClassifyIPs(host, ips, opts.GCP)...)
			}
		}
	}

	return lookupErr
}

// retryResolver retries every lookup once when the failure is transient
// (timeout, SERVFAIL, refused). Authoritative answers and cancellations
// return immediately; both attempts share the domain's lookup context.
type retryResolver struct {
	inner resolve.Resolver
}

func retryOnce[T any](lookup func() (T, error)) (T, error) {
	value, err := lookup()
	if classify.IsTransient(err) {
		return lookup()
	}

	return value, err
}

func (r retryResolver) LookupTXT(ctx context.Context, host string) ([]string, error) {
	return retryOnce(func() ([]string, error) { return r.inner.LookupTXT(ctx, host) })
}

func (r retryResolver) LookupMX(ctx context.Context, host string) ([]*net.MX, error) {
	return retryOnce(func() ([]*net.MX, error) { return r.inner.LookupMX(ctx, host) })
}

func (r retryResolver) LookupCNAME(ctx context.Context, host string) (string, error) {
	return retryOnce(func() (string, error) { return r.inner.LookupCNAME(ctx, host) })
}

func (r retryResolver) LookupIP(ctx context.Context, host string) ([]net.IP, error) {
	return retryOnce(func() ([]net.IP, error) { return r.inner.LookupIP(ctx, host) })
}

func (r retryResolver) LookupNS(ctx context.Context, host string) ([]*net.NS, error) {
	return retryOnce(func() ([]*net.NS, error) { return r.inner.LookupNS(ctx, host) })
}

func firstErr(existing, candidate error) error {
	if existing != nil {
		return existing
	}

	return candidate
}
