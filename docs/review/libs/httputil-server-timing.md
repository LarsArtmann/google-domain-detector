# httputil/server_timing — adoption & utilization review

> Reviewed: 2026-11-09 · Module: `github.com/larsartmann/httputil/server_timing` (package `servertiming`)
> Status in google-domain-detector: **NOT IN MODULE GRAPH** (not adopted)
> Audited checkout: `/home/lars/projects/httputil/server_timing` @ HEAD, exactly at newest tag **server_timing/v1.0.3** (2026-10-24; zero dependencies)
> Consumer repo state at review: google-domain-detector @06cad49 (2026-11-09) — verdicts are point-in-time; re-verify citations against HEAD before relying on them

## Verdict

**Do not adopt.** server_timing generates W3C `Server-Timing` response
headers for HTTP handlers — it is a header _emitter_ for servers, with no
parser for reading such headers from responses a client receives.
google-domain-detector writes no HTTP responses anywhere. Even the loose
idea "record per-phase durations" belongs to slog/terminal output, not a
response header.

## What the library is

A zero-dependency single package: `ServerTimingMiddleware()` installs a
concurrency-safe collector into the request context; handlers call
`RecordServerTiming` / `MeasureServerTiming(ctx, name)`; the collector
serializes into the `Server-Timing` header at first write (auto `total` =
time-to-first-byte). Includes nil-safe no-op variants when no middleware is
installed, injection-hardened value escaping (fuzz-tested), and
`WrapServerTiming` manual writer plumbing that delegates
Flusher/Hijacker/ Pusher. Key citations: `server_timing.go:26-258`
(collector, context API), `server_timing.go:369-416` (middleware),
`middleware.go:6`.

## Current usage in google-domain-detector

None — not in the module graph. The project measures per-domain lookup
budgets with `context.WithTimeout` (`internal/scan/scan.go:58`) and needs
nothing beyond that.

## Why we should NOT use it

1. **Requires being the party that writes response headers.** The CLI never
   does. There is no `Server-Timing` consumer either (the one outbound
   fetch, cloud.json, does not emit such headers).
2. **Zero-dependency and clean, but zero overlap.** No client-side API
   exists in the module (verified).

## Capability ↔ project applicability

| Capability                         | Applies? | Reason                                         |
| ---------------------------------- | -------- | ---------------------------------------------- |
| ServerTimingMiddleware + collector | No       | No responses are written                       |
| Record/Measure context helpers     | No       | Request-context-scoped; terminal tool          |
| HeaderValue serialization          | No       | Header emitter; no parser for received headers |

## Version currency (of the audited checkout)

Stable v1 line: tags v1.0.3 (newest), v1.0.2, v1.0.1, v0.12.0, v0.10.0.
HEAD sits exactly at v1.0.3 — no drift. The root httputil module pins
v1.0.3 as its dependency.

## Revisit trigger

Only as part of a hypothetical HTTP-serving mode (see httputil.md); on its
own there is no path to relevance for a batch CLI.

## How to verify

```bash
rg -n 'servertiming|server_timing' --type go . go.mod go.sum   # absent
cd /home/lars/projects/httputil && git tag --sort=-v:refname | rg server_timing | head -5
cd /home/lars/projects/httputil/server_timing && rg -n 'func ' server_timing.go | head -20
```
