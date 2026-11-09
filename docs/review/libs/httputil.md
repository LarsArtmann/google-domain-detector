# httputil — adoption & utilization review

> Reviewed: 2026-11-09 · Module: `github.com/larsartmann/httputil` (root module)
> Status in google-domain-detector: **NOT IN MODULE GRAPH** (not adopted)
> Audited checkout: `/home/lars/projects/httputil` @ HEAD, 35 commits past newest tag **v1.8.0** (2026-11-03; unreleased commits are CI/docs/test hygiene only, no API changes)

## Verdict

**Do not adopt.** httputil is a server-side `net/http` middleware and
server-lifecycle toolkit — 100% of its exported API operates on incoming
requests, `http.ResponseWriter`, or `http.Server`. google-domain-detector
runs no HTTP server. Despite the generic name, the module contains **no
client-side code at all** (verified by grep: zero `http.Client` /
`client.Do` / `NewRequestWithContext` uses in non-test source), so it
cannot even improve the project's single outbound HTTP call (the cloud.json
feed fetch, `internal/ranges/ranges.go:230-253`).

## What the library is

A composable middleware toolkit for servers: ~18 middlewares (CORS,
security headers, CSP nonce, request ID, logging, recovery, timeout,
response compression, request decompression with bomb cap, CSRF via
nosurf, keyed rate limiting with trusted-proxy handling, language
negotiation, metrics), `Chain`/`Compose`/`MiddlewareStack` composition,
`NewServer` graceful-shutdown lifecycle wrapper, health handlers
(`HealthHandler`/`LiveHandler`/`ReadyHandler`), a `ResponseRecorder`, and
an `httpspec` behavioral test suite (19 default specs) for validating
handlers. Errors flow through go-error-family via thin `Code`/`Domain`
wrappers (`code.go:14-133`). Deps: nosurf, go-error-family, go-etag/server,
server_timing, x/time.

Key file citations (local checkout): `middleware.go`, `cors.go:113`,
`security.go:95`, `nonce.go:143`, `requestid.go:69`, `recovery.go:33`,
`timeout.go:12`, `compression.go:269`, `decompression.go:107`,
`csrf.go:573`, `ratelimit_keyed.go:216`, `language.go:445`,
`metrics.go:59`, `server.go:197`, `health.go:30-90`, `httpspec/specs.go`.

## Current usage in google-domain-detector

None — not imported, not in the module graph. The project's only HTTP
surface is one outbound GET in `internal/ranges/ranges.go:230-253` (plain
`http.Client`, 10 s timeout, no UA/retry/ETag) plus `httptest` servers in
`ranges_test.go` for offline feed tests.

## Why we should NOT use it

1. **No server exists to attach middleware to.** The CLI binds no port,
   serves nothing, exits after rendering. Graceful shutdown, health
   endpoints, rate limiting, CSRF, CORS — all inapplicable by construction.
2. **No client utilities either.** The gaps one might want to close on the
   feed fetch (User-Agent header, retry, ETag/If-None-Match conditional
   GET) would all be _client-side_ helpers; none exist in this repo. Its
   `Timeout` is a handler middleware, its `Decompression` middleware
   decompresses _request_ bodies a server receives — neither fits an
   outbound fetch.
3. **Dependency weight:** 4 external deps (nosurf, x/time, go-etag/server,
   go-error-family) of pure server baggage for a hermetic nix single-binary
   build.

## Capability ↔ project applicability

| Capability                           | Applies? | Reason                                                   |
| ------------------------------------ | -------- | -------------------------------------------------------- |
| Middleware stack (18 middlewares)    | No       | No incoming requests exist                               |
| NewServer lifecycle/graceful drain   | No       | Process is one-shot, exits after render                  |
| Health/ready handlers                | No       | Nothing probes this process                              |
| CSRF / CORS / rate limiting / nonce  | No       | Browser-facing concerns; terminal tool                   |
| Response compression / decompression | No       | Server-side body handling only                           |
| httpspec behavioral test suite       | No       | Tests `http.Handler` values; project has none                  |
| Code/Domain error wrappers           | No       | Server-coded mirror of go-error-family (see that report) |

## Version currency (of the audited checkout)

Tags: v1.8.0 (newest), v1.7.0, v1.6.0, v1.5.0, v1.4.2. HEAD is 35 commits
past v1.8.0 — all post-tag commits are CI/docs/test hygiene plus the
endpoints sub-module release; no exported API drift. Not pinned here; no
currency risk.

## Revisit trigger

Only if google-domain-detector ever grows an HTTP serving mode (e.g. a
local web report viewer). Nothing on the roadmap does; the product is a
pipe-friendly terminal tool by profile.

## How to verify

```bash
rg -n 'httputil' --type go . go.mod go.sum    # absent
cd /home/lars/projects/httputil && git tag --sort=-v:refname | head -5
cd /home/lars/projects/httputil && rg -n 'http\.Client|client\.Do\(' --type go -g '!*_test.go'   # no client code
```
