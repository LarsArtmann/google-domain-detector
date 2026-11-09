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
}

// Run scans every domain with a bounded worker pool and returns results in
// input order. Context cancellation aborts pending domains; their results
// carry the cancellation as Err.
func Run(ctx context.Context, res resolve.Resolver, domains []domain.Domain, opts Options) []classify.Result {
	results := make([]classify.Result, len(domains))
	if len(domains) == 0 {
		return results
	}

	workers := opts.Workers
	if workers <= 0 {
		workers = defaultWorkers
	}
	if workers > len(domains) {
		workers = len(domains)
	}

	queue := make(chan int)
	var wg sync.WaitGroup
	for range workers {
		wg.Go(func() {
			for i := range queue {
				dctx, cancel := context.WithTimeout(ctx, opts.Timeout)
				results[i] = ScanDomain(dctx, res, domains[i], opts)
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

	for i := range results {
		if results[i].Domain == "" {
			results[i] = classify.Result{Domain: domains[i].String(), Err: ctx.Err().Error()}
		}
	}
	return results
}

// ScanDomain probes one domain: TXT (SPF, site verification) and MX at the
// apex, NS at the apex, CNAME and IP-range evidence at apex and www. The DKIM
// selector is only queried when MX and SPF were both silent, so proven
// Workspace domains cost no extra lookup. NXDOMAIN on the first lookup
// short-circuits the domain.
func ScanDomain(ctx context.Context, res resolve.Resolver, d domain.Domain, opts Options) classify.Result {
	apex := d.String()
	r := classify.Result{Domain: apex}

	var lookupErr error

	txts, err := res.LookupTXT(ctx, apex)
	if err != nil {
		if dnsErr, ok := errors.AsType[*net.DNSError](err); ok && dnsErr.IsNotFound {
			r.Err = "domain not found: " + err.Error()
			return r
		}
		lookupErr = err
	} else {
		r.Evidences = append(r.Evidences, classify.ClassifyTXT(apex, txts)...)
	}

	if mxs, err := res.LookupMX(ctx, apex); err != nil {
		lookupErr = firstErr(lookupErr, err)
	} else {
		r.Evidences = append(r.Evidences, classify.ClassifyMX(apex, mxs)...)
	}

	if opts.ProbeDKIM && !r.WorkspaceVerdict().Detected {
		if dkim, err := res.LookupTXT(ctx, "google._domainkey."+apex); err == nil {
			r.Evidences = append(r.Evidences, classify.ClassifyDKIM(apex, dkim)...)
		}
		// A missing selector is the normal case, not an error worth reporting.
	}

	if nss, err := res.LookupNS(ctx, apex); err != nil {
		lookupErr = firstErr(lookupErr, err)
	} else {
		r.Evidences = append(r.Evidences, classify.ClassifyNS(apex, nss)...)
	}

	for _, host := range d.Hosts() {
		if target, err := res.LookupCNAME(ctx, host); err == nil {
			r.Evidences = append(r.Evidences, classify.ClassifyCNAME(host, target)...)
		}
		// No CNAME is the normal case for A-record hosts, never an error.

		if opts.GCP != nil {
			if ips, err := res.LookupIP(ctx, host); err != nil {
				lookupErr = firstErr(lookupErr, err)
			} else {
				r.Evidences = append(r.Evidences, classify.ClassifyIPs(host, ips, opts.GCP)...)
			}
		}
	}

	if lookupErr != nil && len(r.Evidences) == 0 {
		r.Err = lookupErr.Error()
	}
	return r
}

func firstErr(existing, candidate error) error {
	if existing != nil {
		return existing
	}
	return candidate
}
