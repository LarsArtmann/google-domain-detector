# go-retry — adoption & utilization review

> Reviewed: 2026-11-09 · Module: `github.com/larsartmann/go-retry`
> Status in google-domain-detector: **NOT IN MODULE GRAPH** (not adopted; STACK.md exclusion on record)
> Audited checkout: `/home/lars/projects/go-retry` @ `047d55a` = v0.9.0 + 4 chore-only commits (README/codeql/docs)

## Verdict

**Uphold the exclusion — do not adopt.** STACK.md's recorded reason
("`net.Resolver` retries internally; a batch tool re-runs cheaply") holds.
The one theoretically retryable call, the cloud.json fetch, runs **once per
24 h window** against one of the most reliable CDNs on earth, is bounded by
a 10 s client timeout, and already has a stale-cache fallback
(ranges.go:183-185). Wrapping it in a retry policy adds a dependency to
defend against a near-nonexistent failure mode. Bonus finding: the
library's fail-open defaults (unclassified errors retry) make it a footgun
unless paired with go-error-family — which this project deliberately
doesn't adopt (see go-error-family.md).

## What the library is

`retry.Do(ctx, op, opts...)` with: configurable Attempts/MaxAttempts +
wall-clock deadline, 6 backoff strategies (Fixed/Exponential/Polynomial/
Linear/Jittered/Decorrelated) with cap and max-elapsed, context-aware
cancellation, per-attempt timeout enforcement, error accumulation
(`Errors()`), `ShouldRetry(predicate)` — including an
`errorfamily.ShouldRetry` classifier — and `OnRetry`/`OnAttempt` hooks
(retry.go:70-260, options.go, strategy.go, predicate.go). Deps: stdlib +
go-error-family.

## Current usage in google-domain-detector

None — not imported, not in the graph.

## Why we should NOT use it

1. **The DNS path already retries below us.** Go's `net.Resolver` performs
   its own query retries per lookup; a per-domain timeout
   (scan.go:58) bounds the budget. Retrying at the scan layer would
   multiply load against uncooperative resolvers.
2. **The HTTP path is once-per-day** with a stale-cache fallback; a user
   re-running the tool IS the retry.
3. **Fail-open hazard:** without ShouldRetry predicates, an unclassifiable
   error (e.g. an unexpected status from a misconfigured --ranges-url)
   retries uselessly; classification requires go-error-family adoption.

## Capability ↔ project applicability

| Capability                    | Applies? | Reason                                             |
| ----------------------------- | -------- | -------------------------------------------------- |
| Do + Attempts/Deadline        | No       | No call justifying a retry loop                     |
| Backoff strategies (6)        | No       | Same                                               |
| Per-attempt timeout           | No       | Client timeout + ctx already bound the fetch        |
| errorfamily.ShouldRetry       | No       | No error-family adoption (see that report)          |
| OnRetry/OnAttempt hooks       | No       | No retry events to observe                         |

## Version currency (of the audited checkout)

Tags: v0.9.0 (newest), v0.8.0, v0.7.0, v0.6.0, v0.5.1. HEAD = v0.9.0 +
chores. Healthy; rejection is fit.

## Revisit trigger

If the tool ever schedules itself (daemon/cron mode with unattended
operation), a single bounded retry (2 attempts, jittered) around the feed
fetch becomes reasonable. That mode is currently rejected by product
profile (see go-daemon.md).

## How to verify

```bash
rg -n 'go-retry' --type go . go.mod go.sum    # absent
cd /home/lars/projects/go-retry && git tag --sort=-v:refname | head -5
cd /home/lars/projects/go-retry && rg -n 'ShouldRetry' predicate.go errorfamily/*.go
```
