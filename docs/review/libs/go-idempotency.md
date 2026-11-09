# go-idempotency — adoption & utilization review

> Reviewed: 2026-11-09 · Module: `github.com/larsartmann/go-idempotency`
> Status in google-domain-detector: **NOT IN MODULE GRAPH** (not adopted)
> Audited checkout: `/home/lars/projects/go-idempotency` @ `9a82577` = v0.3.2 + 5 commits (SDK + NixOS module chores; **new `contract/` package unreleased**)

## Verdict

**Do not adopt.** go-idempotency implements Idempotency-Key request
deduplication for HTTP APIs: middleware that caches responses by key,
scenarios (in-flight, replay, conflict), `Idempotency-Key` + throwaway
`RequestId` header conventions, fs-backed JSON journal, and Prometheus
metrics. It presumes (a) an inbound HTTP API and (b) **non-idempotent
operations being retried by clients**. google-domain-detector is a
terminal CLI with no server, no clients, and no side effects at all —
a re-run of a read-only scan is already idempotent by nature.

## What the library is

Verified in the checkout: `Manager`/`Scenario` types (Manager,
ScenarioInFlight, ScenarioReplay, ScenarioConflict), header constants
(idempotency.go:30-45), `NewMiddleware` (middleware.go:20-50), fs-backed
JSON journal (journal.go), metrics module, NixOS module, throwaway-key
conventions (throwaway.go), docs/examples with curl.

## Current usage in google-domain-detector

None — not imported, not in the graph.

## Why we should NOT use it

1. No inbound HTTP surface (same server-mismatch as httputil.md).
2. No mutating operations exist to deduplicate; dedup of identical scan
   inputs is already handled by input dedup (internal/input) at zero cost.
3. Header conventions are for HTTP request/response contracts, not CLI
   invocations.

## Capability ↔ project applicability

| Capability            | Applies? | Reason                                     |
| --------------------- | -------- | ------------------------------------------ |
| Key-based dedup middleware | No   | No server, no requests                     |
| In-flight/replay/conflict scenarios | No | No concurrent callers of a write op |
| fs journal            | No       | Nothing journaled                          |
| Prometheus metrics    | No       | No metrics endpoint                        |

## Version currency (of the audited checkout)

Tags: v0.3.2 (newest), v0.3.1, v0.3.0, v0.2.0, v0.1.0. HEAD = v0.3.2 + 5
commits (SDK + NixOS chores; `contract/` package not yet tagged). Healthy;
rejection is fit.

## Revisit trigger

Only if an HTTP API mode with mutating endpoints ever exists (rejected as
a product direction across httputil/cqrs-htmx/go-appkit reports).

## How to verify

```bash
rg -n 'idempotency' --type go . go.mod go.sum    # absent
cd /home/lars/projects/go-idempotency && git tag --sort=-v:refname | head -5
cd /home/lars/projects/go-idempotency && rg -n 'Scenario' idempotency.go | head
```
