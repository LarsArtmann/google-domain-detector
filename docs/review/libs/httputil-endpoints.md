# httputil/endpoints — adoption & utilization review

> Reviewed: 2026-11-09 · Module: `github.com/larsartmann/httputil/endpoints`
> Status in google-domain-detector: **NOT IN MODULE GRAPH** (not adopted)
> Audited checkout: `/home/lars/projects/httputil/endpoints` @ HEAD, exactly at newest tag **endpoints/v0.1.0** (2026-11-02)
> Consumer repo state at review: google-domain-detector @d07a343 (2026-11-09) — verdicts are point-in-time; re-verify citations against HEAD before relying on them

## Verdict

**Do not adopt.** endpoints provides self-describing HTTP route registration
for Go's `http.ServeMux`: routes registered through its `Registry` are both
mounted on the mux and recorded, and a mounted `GET /endpoints` listing
serves them as JSON (for API clients) or an HTML table (for humans), picked
by `Accept`-header content negotiation — the listing can never drift from
what is actually served. The detector registers no HTTP routes and serves
nothing, so there is nothing to list.

## What the library is

Verified against the checkout (endpoints.go, negotiate.go, qvalue.go,
render.go):

- `NewRegistry()` → `Handle(mux, pattern, handler, summary)` /
  `HandleFunc(...)` — one declaration mounts AND records; wiring errors are
  values, never panics (nil mux, empty pattern, mux-rejected pattern).
- `Register(mux, registry)` mounts `GET /endpoints` (the listing includes
  itself); `Routes()` / `Handler()` expose the record programmatically.
- Content negotiation over `Accept` with a hand-rolled, allocation-free
  q-value parser (qvalue.go:28-114); JSON and HTML renderers (render.go).
- Zero dependencies — stdlib-only, usable without the rest of httputil.

## Current usage in google-domain-detector

None — not imported, not in the graph.

## Why we should NOT use it

1. **Requires an HTTP mux with routes.** Its whole value is keeping a
   route listing in lockstep with a server; a batch CLI has neither.
2. The closest analogy in this project — the `--format`/`--only` value
   enums — is already enforced by cmdguard's `values:` tag +
   `output.ParseFormat` (internal/cli/cli.go:38,114).

## Capability ↔ project applicability

| Capability                       | Applies? | Reason                   |
| -------------------------------- | -------- | ------------------------ |
| Registry + Handle/HandleFunc     | No       | No routes to register    |
| GET /endpoints JSON/HTML listing | No       | No server to mount it on |
| Accept content negotiation       | No       | No HTTP responses        |
| Q-value parser                   | No       | Nothing to negotiate     |

## Version currency (of the audited checkout)

Tag: `endpoints/v0.1.0` (newest and only release). HEAD is exactly at the
tag — zero drift. Young but clean; rejection is fit, not quality.

## Revisit trigger

Only alongside the HTTP-serving mode already ruled out in httputil.md.

## How to verify

```bash
rg -n 'httputil/endpoints' --type go . go.mod go.sum   # absent
cd /home/lars/projects/httputil && git tag --sort=-v:refname | rg endpoints
cd /home/lars/projects/httputil/endpoints && rg -n 'func NewRegistry|func Register' endpoints.go
```
