# go-etag — adoption & utilization review

> Reviewed: 2026-11-09 · Module: `github.com/larsartmann/go-etag` (root) — real code lives in sub-modules `go-etag/server`, `go-etag/client`, `go-etag/entitytag`, `go-etag/metrics`
> Status in google-domain-detector: **NOT IN MODULE GRAPH** (not adopted)
> Audited checkout: `/home/lars/projects/go-etag` @ `226fa97` = v0.7.0 + 1 CI-only commit. **Root module is a deprecated alias shim** (etag.go:1-9, "will be removed in a future release") re-exporting go-etag/server symbols.

## Verdict

**Do not adopt.** The one scenario with theoretical pull — conditional
re-fetch of the GCP ranges feed (`If-None-Match` + 304) — is not actually
served by this library's client module: `client.NewInMemoryClient` is a
**memory-only** ETag store for test doubles, with zero persistence
(client/client.go:1-40; server/*_test.go is its only consumer). It has no
disk cache, no HTTP-cache semantics, and the server side is for generating
ETags in responses — a server the tool doesn't have. The feed is fetched at
most once per 24 h; a 304 saves one ~100 KiB download per day. Not worth a
dependency even if the mechanism fit.

## What the library is

- `go-etag/server` — handler middleware + helpers producing strong/weak
  ETags (SHA-256/Murmur/XXH3/size/length/crc32, count-based, composite,
  time-based rounded), writing `ETag`/`If-None-Match` handling into
  responses, gzip-aware.
- `go-etag/client` — in-memory client store + `NewETagHandler` middleware:
  attach stored ETag to outbound requests, persist it on response.
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

| Capability                | Applies? | Reason                                            |
| ------------------------- | -------- | ------------------------------------------------- |
| Server ETag generation    | No       | No responses written                               |
| client (in-memory) store  | No       | memory-only; process exits after each run          |
| entitytag Parse/Compare   | No       | no ETag parsing need                               |
| metrics                   | No       | no Prometheus surface                              |
| (hand-rolled alternative) | —        | 30 lines in ranges.go if ever needed               |

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
cd /home/lars/projects/go-etag && head -12 etag.go          # deprecation shim
cd /home/lars/projects/go-etag && rg -n 'InMemory|os\.|WriteFile' client/*.go | head   # memory-only client
```
