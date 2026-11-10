# go-datastar — adoption & utilization review

> Reviewed: 2026-11-09 · Module: `github.com/larsartmann/go-datastar` (multi-module: root + `broadcast`, `datastartest`, `static` per go.work)
> Status in google-domain-detector: **NOT IN MODULE GRAPH** (not adopted)
> Audited checkout: `/home/lars/projects/go-datastar` @ `39e415f` = `broadcast/v0.7.0` + 4 commits (security fix, tests, docs — API surface unchanged)
> Consumer repo state at review: google-domain-detector @d07a343 (2026-11-09) — verdicts are point-in-time; re-verify citations against HEAD before relying on them

## Verdict

**Do not adopt.** go-datastar is the Go SDK for the DataStar reactive-UI
browser library: server-side helpers that emit SSE-delivered patches
(patches, script execution, URL replacement, signal read/write), a
broadcast hub with replay and heartbeats, and an embedded copy of the
DataStar JS runtime. It requires an HTTP server and a browser;
google-domain-detector has neither. Its entire output surface is terminal
table/JSON/CSV/TSV/Markdown via go-output.

## What the library is

Verified in the checkout:

- **root** — typed patches (`ScriptPatch`, `NewRedirectPatch`,
  `NewConsoleLog(Patch)`, `NewDispatchCustomEventPatch`,
  `NewReplaceURLPatch`, script_convenience.go:16-182); response/event
  plumbing (response.go, script_handler.go, patch.go); inbound signal
  reading (`ReadSignals` from query/body, `LastEventID`, inbound.go:26-107);
  signal store (store.go, signals.go); `GetSSE`/`PostSSE`/... fetch helpers
  (http.go:6-26); generated constants mirroring DataStar event/attribute
  names (constants.go); DataStar version constraint checking.
- **broadcast** — `Broadcaster` + `Store` interface with
  `WithBufferSize`/`WithReplayCapacity`/`WithStore`/`WithHeartbeatInterval`
  (options.go:25-106) for cross-request signal broadcasts.
- **static** — `Bytes()` returns the go:embed'ed DataStar JS runtime
  (static.go:29) so binaries can serve it themselves. Tags: static/v0.4.0
  family (no `push/` module — assets live in `static`).
- **datastartest** — test kit for handler-level assertions.

## Current usage in google-domain-detector

None — not imported, not in the graph.

## Why we should NOT use it

1. **Requires the triad server + SSE + browser.** Every patch type exists
   to be delivered as `text/event-stream` to a DataStar-enabled page. A
   terminal CLI has no page, no JS runtime, no connection.
2. **Not a data-layer library.** "Signals" are UI-state primitives (typed
   browser↔server bindings), not general-purpose state a CLI could borrow.
3. Wrong dependency direction too: adopting it would pull go-sse + friends
   into a binary that renders plain text.

## Capability ↔ project applicability

| Capability                          | Applies? | Reason                          |
| ----------------------------------- | -------- | ------------------------------- |
| Patch emission (script/URL/console) | No       | No SSE responses to patch       |
| Signal read/write/store             | No       | Browser-state primitives        |
| broadcast hub + replay              | No       | Multi-client fan-out            |
| static JS embedding                 | No       | Serving JS to browsers          |
| datastartest kit                    | No       | No DataStar handlers under test |

## Version currency (of the audited checkout)

Latest tags: `broadcast/v0.7.0`, `static/v0.4.0`, root `v0.7.0` family.
HEAD is 4 commits past broadcast/v0.7.0 (security fix + tests). Healthy;
rejection is fit.

## Revisit trigger

Only if a web dashboard mode is ever built — a direction rejected across
the cqrs-htmx / templ-components / go-appkit reports.

## How to verify

```bash
rg -n 'datastar' --type go . go.mod go.sum    # absent
cd /home/lars/projects/go-datastar && git tag --sort=-creatordate | head -5
cd /home/lars/projects/go-datastar && cat go.work               # . ./broadcast ./datastartest ./static
cd /home/lars/projects/go-datastar && rg -n 'func Bytes' static/static.go
```
