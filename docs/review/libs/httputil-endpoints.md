# httputil/endpoints — adoption & utilization review

> Reviewed: 2026-11-09 · Module: `github.com/larsartmann/httputil/endpoints`
> Status in google-domain-detector: **NOT IN MODULE GRAPH** (not adopted)
> Audited checkout: `/home/lars/projects/httputil/endpoints` @ HEAD, exactly at newest tag **endpoints/v0.1.0** (2026-11-02)

## Verdict

**Do not adopt.** endpoints is a **comptime code-generation framework for
`http.Handler` registrations**: its `endpoints.Enforce[M, H]()` generic
hook makes a test FAIL at build time when a manifest struct and the real
handler map drift apart. The detector has zero handlers, zero endpoints,
zero manifests — the entire mechanism has no input to operate on.

## What the library is

Single package, zero dependencies (verified):

- Pattern: you keep a `map[string]http.Handler` and a parallel manifest
  struct; each handler implementation embeds
  `endpoints.EndpointDef[ManifestField]`.
- `endpoints.Enforce[M, H]()` — a no-op at runtime that constrains `H` to
  implement `Endpoint[Manifest[M]]` at compile time; a generated test
  (`generator.Main`, `endpoints check`) re-runs `go test -run` per gap and
  fails with a pinpointed message naming the missing handler field.
- `endpoints check` CLI + `endpoints generate` (scaffolds the test), wired
  through `internal/generator` (generator/main.go:44-93,
  internal/tests.go:52-140).
- An optional VFS layer + testify-based `ShouldBeServed` test helper.

Tagline: "comptime codegen with zero dependencies". Verified: no runtime
deps; a single `go test` indirect.

## Current usage in google-domain-detector

None — not imported, not in the graph.

## Why we should NOT use it

1. **Mechanism requires an HTTP surface.** Its one job is keeping a route
   table and handler implementations in lockstep. A CLI with no routes has
   nothing to enforce.
2. The closest analogy in this project — "flag values enum must match
   implemented renderers" — is already enforced better by cmdguard's
   `values:` tag plus `output.ParseFormat` validation (internal/cli/cli.go:38,114).

## Capability ↔ project applicability

| Capability                  | Applies? | Reason                                        |
| --------------------------- | -------- | --------------------------------------------- |
| Endpoint/handler drift tests | No      | No handlers                                   |
| Manifest scaffolding CLI    | No       | No manifests                                  |
| testify serving assertions  | No       | testify not used; nothing served             |

## Version currency (of the audited checkout)

Tag: `endpoints/v0.1.0` (newest; only release). HEAD is exactly at the tag
— zero drift. Young but clean; rejection is fit.

## Revisit trigger

Only alongside the HTTP-serving mode already ruled out in httputil.md.

## How to verify

```bash
rg -n 'endpoints' --type go . go.mod go.sum    # absent (check for the httputil path specifically)
cd /home/lars/projects/httputil && git tag --sort=-v:refname | rg endpoints
cd /home/lars/projects/httputil/endpoints && rg -n 'func Enforce' *.go internal/*.go
```
