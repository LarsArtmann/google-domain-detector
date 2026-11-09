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
defend against a near-nonexistent failure mode. Bonus finding: its default
predicate is `errorfamily.IsRetryable` (go-retry/retry.go:106-108), which
delegates to go-error-family's `Classify` — and that classifier **defaults
unknown errors to Transient** (classify.go, "fail-open for retry"), so
unclassified errors retry by default; tightening requires `WithIsRetryable`
or wrapping errors with explicit families.

## What the library is

`retry.Do(ctx, op, opts...)` with: configurable Attempts/MaxAttempts +
wall-clock deadline, backoff strategies (incl. jittered/decorrelated) with
cap and max-elapsed, context-aware cancellation, per-attempt timeout
enforcement, error accumulation (`Errors()`), `Config.IsRetryable` /
`WithIsRetryable(f)` predicate — defaulting to
`errorfamily.IsRetryable` (only-Transient, but fail-open via Classify's
unknown→Transient default) — and OnRetry/OnAttempt hooks (retry.go:42-119,
options.go:17-26). Deps: stdlib + go-error-family.

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
| Backoff strategies            | No       | Same                                               |
| Per-attempt timeout           | No       | Client timeout + ctx already bound the fetch        |
| IsRetryable predicate         | No       | No retry loop to gate                              |
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
cd /home/lars/projects/go-retry && rg -n 'IsRetryable' retry.go options.go
```
