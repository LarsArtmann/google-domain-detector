# go-health — adoption & utilization review

> Reviewed: 2026-11-09 · Module: `github.com/larsartmann/go-health` (+ sub-modules aggregators, checks, http, live, ctx, responsewriter)
> Status in google-domain-detector: **NOT IN MODULE GRAPH** (not adopted)
> Audited checkout: `/home/lars/projects/go-health` @ `7e5d8d1` = v0.10.0 + 1 docs-only commit

## Verdict

**Do not adopt.** go-health is a health-probe framework for long-running
services: a Registry of Check functions, aggregation (all/any/threshold),
`/healthz`-style HTTP handlers with configurable response codes, live
variable snapshots (expvar-style), and an aggregator for multiple
registries. google-domain-detector has no process to probe: it starts,
scans, renders, exits in seconds. There is no registry to keep, no
readiness state to expose, no poller to answer.

## What the library is

Core `Registry` + `Check{}`/`CheckFunc` + `NewRegistry`/`MustRegistry`,
results with Status/Duration/Error/Timestamp; 5 aggregation strategies
(All, Any, Count of healthy, Failover, Criticality-weighted
`NewFailureThreshold`); HTTP handlers `NewHandler`/`NewReadinessHandler`
with configurable status mappings; `live` package (live.go:32-98):
atomic live values, groups, snapshots with TTL, function-backed values;
HTTP-level error rate/latency trackers built on a wrap-ResponseWriter;
per-check context timeout enforcement. Verified in the checkout
(health.go, registry.go, aggregator.go, http.go, live.go).

## Current usage in google-domain-detector

None — not imported, not in the graph.

## Why we should NOT use it

1. **Nothing probes this process.** Health endpoints exist so orchestrators
   can ask "are you ready?" — meaningless for a foreground CLI whose
   lifetime is one scan.
2. The programmatic core (Registry + checks) without HTTP still assumes a
   long-lived registry consulted repeatedly; the detector's closest analog
   — "did the ranges feed load?" — is a single `Source` return value it
   already has (ranges.go:148-188).
3. cmdguard's `doctor` battery already covers the one-shot diagnostic
   angle (see cmdguard.md gap #2: `WithDoctorCheck` preflight).

## Capability ↔ project applicability

| Capability                    | Applies? | Reason                                          |
| ----------------------------- | -------- | ----------------------------------------------- |
| Registry + Check functions    | No       | No long-lived probe state                        |
| Aggregation strategies        | No       | Nothing to aggregate over time                  |
| HTTP handlers (/healthz)      | No       | No server                                        |
| live variable snapshots       | No       | No observability surface                        |
| HTTP error-rate/latency trackers | No    | No requests served                              |

## Version currency (of the audited checkout)

Tags: v0.10.0 (newest), v0.9.1, v0.9.0, v0.8.0, v0.7.1. HEAD = v0.10.0 +
docs. Healthy; rejection is fit.

## Revisit trigger

Only in the rejected server/daemon modes (go-daemon.md, go-appkit.md).

## How to verify

```bash
rg -n 'go-health' --type go . go.mod go.sum    # absent
cd /home/lars/projects/go-health && git tag --sort=-v:refname | head -5
cd /home/lars/projects/go-health && ls */    # aggregators checks ctx http live responsewriter
```
