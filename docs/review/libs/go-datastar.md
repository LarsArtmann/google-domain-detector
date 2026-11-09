# go-datastar — adoption & utilization review

> Reviewed: 2026-11-09 · Module: `github.com/larsartmann/go-datastar` (multi-module: root + broadcast/v0.7.0 + push/v0.4.0)
> Status in google-domain-detector: **NOT IN MODULE GRAPH** (not adopted)
> Audited checkout: `/home/lars/projects/go-datastar` @ `39e415f` = broadcast/v0.7.0 + 4 commits (security fix, tests, docs — API surface unchanged)

## Verdict

**Do not adopt.** go-datastar is a full-stack reactive-UI SDK for the
DataStar browser library: server-side Go helpers that emit SSE-delivered
signal patches and execute expressions, plus a static asset pipeline. It
requires an HTTP server and a browser. google-domain-detector has neither;
its entire output surface is a terminal table / JSON / CSV / TSV / Markdown
via go-output.

## What the library is

Three modules (verified in the checkout):

- **root** — DataStar SDK: `WithSSE`/`WithSSEOptions` handler setup,
  `ExecuteScript`, `MergeFragments`, `MergeSignals`, `RemoveSignals`,
  `signals.MarshalGob`/`UnmarshalGob` for state snapshots, context helpers,
  circuit-breaker client with 3 retries, and generated Go constants
  mirrors of DataStar's event/attribute names (~350 constants, events.go).
- **broadcast** — `Broker` for cross-request/cross-goroutine signal
  broadcasts; the reason go-sse grew `ssegen` (test generators).
- **push** — `PushStaticAssets` embeds a configurable subset of the
  DataStar JS runtime (with SRI hash pinning) so Go binaries serve it
  themselves.

Reactivity is opt-in per-handler; the root module is a library (no
middleware), with go-sse as its stream transport.

## Current usage in google-domain-detector

None — not imported, not in the graph.

## Why we should NOT use it

1. **Requires the triad server + SSE + browser.** Every entry point writes
   to an `http.ResponseWriter` as `text/event-stream` for a DataStar-enabled
   page. A terminal CLI has no page, no JS runtime, no connection.
2. **Not a data-layer library.** Despite "signals" sounding data-ish, the
   signal APIs are UI-state primitives (typed GET/POST bindings between
   browser state and server), not general-purpose state management a CLI
   could borrow.
3. Dependency direction is wrong too: adopting it would pull go-sse +
   `charmbracelet/x` into a binary that renders plain text.

## Capability ↔ project applicability

| Capability                     | Applies? | Reason                                    |
| ------------------------------ | -------- | ----------------------------------------- |
| SSE handler helpers            | No       | No server                                  |
| Signal merge/remove/execute    | No       | Browser-state primitives                   |
| Signals gob snapshots          | No       | UI state persistence                       |
| broadcast Broker               | No       | Multi-client fan-out                       |
| push static assets (SRI)       | No       | Serving JS to browsers                     |

## Version currency (of the audited checkout)

Tags: `broadcast/v0.7.0` (newest), `push/v0.4.0`, root `v0.7.0`, `v0.6.1`,
`v0.6.0`. HEAD is 4 commits past broadcast/v0.7.0 (security fix + tests).
Healthy; rejection is fit.

## Revisit trigger

Only if a web dashboard mode is ever built (rejected as a product direction
in the cqrs-htmx and templ-components reports; the same verdict applies).

## How to verify

```bash
rg -n 'datastar' --type go . go.mod go.sum    # absent
cd /home/lars/projects/go-datastar && git tag --sort=-creatordate | head -5
cd /home/lars/projects/go-datastar && rg -n 'text/event-stream' --type go -g '!*_test.go' | head -5
```
