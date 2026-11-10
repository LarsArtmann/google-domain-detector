# go-etag — adoption & utilization review

> Reviewed: 2026-11-09 · Module: `github.com/larsartmann/go-etag` (root) — real code lives in sub-modules `go-etag/server`, `go-etag/client`, `go-etag/entitytag`, `go-etag/metrics`
> Status in google-domain-detector: **NOT IN MODULE GRAPH** (not adopted)
> Audited checkout: `/home/lars/projects/go-etag` @ `226fa97` = v0.7.0 + 1 CI-only commit. **Root module is deprecated** (deprecated.go — re-exports
> go-etag/server symbols, slated for removal).
> Consumer repo state at review: google-domain-detector @f5ea3ac (2026-11-09) — verdicts are point-in-time; re-verify citations against HEAD before relying on them

## Verdict

**Do not adopt.** The one scenario with theoretical pull — conditional
re-fetch of the GCP ranges feed (`If-None-Match` + 304) — is half-served
by this library at best: the client module is a real caching
`http.RoundTripper` (`NewTransport`, transport.go:44), but its validator
cache is a bounded **in-process** `responseCache` (cache.go:67-118) with no
disk persistence and no pluggable cache backend — and a one-shot CLI's
process dies after every run, so cached ETags never survive to the next
fetch. The server side generates ETags for responses the tool never
writes. The feed is fetched at most once per 24 h; a 304 saves one ~100 KiB
download per day. Not worth a dependency.

## What the library is

- `go-etag/server` — handler middleware + helpers producing strong/weak
  ETags (SHA-256/Murmur/XXH3/size/length/crc32, count-based, composite,
  time-based rounded), writing `ETag`/`If-None-Match` handling into
  responses, gzip-aware.
- `go-etag/client` — `NewTransport(next http.RoundTripper, opts)` caching
  transport: attaches stored validators to outbound GETs, rebuilds cached
  responses from 304s, freshens via HEAD, exposes hit/miss `Stats`
  (transport.go:36-262); backed by the in-process bounded `responseCache`
  (cache.go:67-118) — memory only, not pluggable.
- `go-etag/entitytag` — `Parse`/`Compare`/`StrongCompare` RFC 9110 §8.8.3
  logic, wildcard support (entitytag.go:10-70).
- `go-etag/metrics` — Prometheus collectors for hit/miss/matched.

## Current usage in google-domain-detector

None — not imported, not in the graph. The feed fetch
(internal/ranges/ranges.go:230-253) does a plain unconditional GET.

## Why we should NOT use it

1. **Mechanism mismatch:** no persistent client store exists; wiring this
   would require writing a disk-backed client implementation against an
   interface whose only in-tree implementation is a test fake.
2. **Value ≈ nil:** once-per-24 h fetch, ~100 KiB body, stale-cache
   fallback already in place. Bandwidth saved: negligible.
3. **Server-side half inapplicable:** the tool writes no responses.
4. If conditional fetch were ever wanted, it is ~30 hand-rolled lines
   (store ETag beside the cache file, send If-None-Match, treat 304 as
   cache-refresh) — smaller than the dependency.

## Capability ↔ project applicability

| Capability                | Applies? | Reason                                          |
| ------------------------- | -------- | ----------------------------------------------- |
| Server ETag generation    | No       | No responses written                            |
| client caching transport  | No       | in-process cache dies with the one-shot process |
| entitytag Parse/Compare   | No       | no ETag parsing need                            |
| metrics                   | No       | no Prometheus surface                           |
| (hand-rolled alternative) | —        | 30 lines in ranges.go if ever needed            |

## Version currency (of the audited checkout)

Root tags: v0.7.0 (newest), v0.6.1, v0.6.0, v0.5.0, v0.4.0. Note the root
module is deprecated upstream (alias shim); any future adoption should
target `go-etag/client` or `/server` sub-modules directly. Rejection makes
this moot.

## Revisit trigger

Only if the feed were polled frequently (daemon mode) AND Google's CDN
ETags proved stable for this URL. Both premises are currently false.

## How to verify

```bash
rg -n 'go-etag' --type go . go.mod go.sum    # absent
cd /home/lars/projects/go-etag && head -12 deprecated.go          # root deprecation
cd /home/lars/projects/go-etag && rg -n 'func NewTransport|responseCache' client/transport.go client/cache.go | head
```
