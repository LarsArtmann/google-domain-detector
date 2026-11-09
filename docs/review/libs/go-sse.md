# go-sse — adoption & utilization review

> Reviewed: 2026-11-09 · Module: `github.com/larsartmann/go-sse`
> Status in google-domain-detector: **NOT IN MODULE GRAPH** (not adopted)
> Audited checkout: `/home/lars/projects/go-sse` @ `443abdd` = v0.7.0 + 1 test-only commit
> Deps: a-h/templ, go-branded-id, go-error-family (not stdlib-only)

## Verdict

**Do not adopt.** go-sse is a full Server-Sent Events stack: a generic
broadcast fan-out, an HTTP `Stream` writer, Last-Event-ID replay, a WHATWG
`text/event-stream` parser with a conformance corpus, and handler test
helpers. Serving SSE requires an HTTP handler; consuming it requires a
streaming response. google-domain-detector does neither — its only HTTP
call (`internal/ranges/ranges.go:230-253`) downloads one bounded JSON
document and closes.

## What the library is

Verified in the checkout:

| Package    | What it provides                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                          |
| ---------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| root `sse` | branded `EventID` (go-branded-id); `Event` + wire writers `WriteEvent`/`WriteHeartbeat`/`WriteRetry`/`WriteKeyedLines` (event.go:167-351); generic `Broadcaster[T]` fan-out — Subscribe/SubscribeFilter/Unsubscribe, Broadcast(BroadcastMany), drop callbacks (`WithOnDrop`), buffer sizing, `Health()`, graceful `Shutdown(ctx)` (broadcaster.go:68-108, fanout.go); `Stream` — `NewStream(w, r)` HTTP response writer with Send/SendData/SendJSON/SendLines (stream.go:38-192); `EventStore`/`FilteredEventStore` + `Replay`/`ReplayFiltered` for Last-Event-ID resume (replay.go:9-81) |
| `sseparse` | reader-side parser: `ReadEvents`/`ReadNEvents` with `WithMaxLineBytes` bound, `FindByType`/`FilterByType`, plus a WHATWG conformance corpus (`Corpus()`, sseparse/corpus.go:29-88)                                                                                                                                                                                                                                                                                                                                                                                                        |
| `ssetest`  | handler test kit: `Collect`/`CollectPost`/`CollectWithRequest` against an `http.Handler` with `WithPath`/`WithHeader`/`WithLastEventID` options (ssetest/collect.go, options.go)                                                                                                                                                                                                                                                                                                                                                                                                          |

## Current usage in google-domain-detector

None — not imported, not in the graph.

## Why we should NOT use it

1. **No stream in either direction.** No handler to `Stream` from, no
   broadcaster subscribers, no event-stream input to `sseparse`.
2. Deps (templ, go-branded-id, go-error-family) would enter the binary for
   zero used surface.
3. Effort to adopt: S mechanically; value: zero. YAGNI by construction.

## Capability ↔ project applicability

| Capability               | Applies? | Reason                     |
| ------------------------ | -------- | -------------------------- |
| Stream (HTTP SSE writer) | No       | No responses written       |
| Broadcaster[T] fan-out   | No       | No subscribers             |
| Replay / EventStore      | No       | No resume semantics        |
| sseparse reader + corpus | No       | No event-stream input      |
| ssetest Collect helpers  | No       | No SSE handlers under test |

## Version currency (of the audited checkout)

Tags: v0.7.0 (newest), v0.6.1, v0.6.0, v0.5.0, v0.4.0. HEAD = v0.7.0 plus
one test-only commit. Healthy; rejection is fit.

## Revisit trigger

Only if the tool ever consumes or serves event streams (live progress to a
web UI) — a direction already rejected across the httputil/go-datastar/
cqrs-htmx reports.

## How to verify

```bash
rg -n 'go-sse|sseparse|ssetest' --type go . go.mod go.sum   # absent
cd /home/lars/projects/go-sse && git tag --sort=-v:refname | head -5
cd /home/lars/projects/go-sse && ls */                        # sseparse ssetest
```
