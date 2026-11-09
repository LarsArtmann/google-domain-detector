# go-sse — adoption & utilization review

> Reviewed: 2026-11-09 · Module: `github.com/larsartmann/go-sse`
> Status in google-domain-detector: **NOT IN MODULE GRAPH** (not adopted)
> Audited checkout: `/home/lars/projects/go-sse` @ `443abdd` = v0.7.0-1 (v0.7.0 plus one test-only commit)

## Verdict

**Do not adopt.** go-sse is a production Server-Sent Events stack for Go
HTTP servers: SSE handler/broadcaster for serving streams, plus a
WHATWG-compliant parser for consuming them. google-domain-detector runs no
server, makes no streaming requests, and terminates after rendering a
report. Zero of its four packages has a caller here.

## What the library is

Four packages (verified in the checkout):

| Package    | What it provides                                                                    |
| ---------- | ----------------------------------------------------------------------------------- |
| root `sse` | SSE handler + broadcaster: Subscribe/Unsubscribe, Broadcast + Broadcastf, message hooks, graceful Close (event.go:40-92, hub.go:14-81, handler.go:14-46) |
| `sseparse` | Streaming byte-at-a-time WHATWG `text/event-stream` parser with PLAIN-objects mode for zero-allocation loops (parser.go:47-172) |
| `ssegen`   | Round-trip SSE event generator for tests (fake server side), fuzz-seeded (ssegen.go:12-90) |
| `ssecycle` | Deterministic event generator driving week-long simulated SSE cycles for soak tests (ssecycle.go:15-190) |

Runtime deps: only stdlib. Its own test tooling depends on
charmbracelet/x/exp/teatest (tests only).

## Current usage in google-domain-detector

None — not imported, not in the graph.

## Why we should NOT use it

1. **No stream in either direction.** Serving SSE requires an HTTP handler;
   consuming SSE requires a long-lived streaming response. The tool's only
   HTTP call (`internal/ranges/ranges.go:230-253`) downloads a bounded
   (~100 KiB, 10 MiB cap) JSON document and closes.
2. Even the parser package, the only client-side-adjacent piece, has no
   input to parse — no feed, API, or service the tool talks to emits
   `text/event-stream`.
3. Effort to adopt: S mechanically (import + wire); value: zero. YAGNI by
   construction.

## Capability ↔ project applicability

| Capability            | Applies? | Reason                                     |
| --------------------- | -------- | ------------------------------------------ |
| SSE handler/broadcaster | No     | No server, no subscribers                  |
| Streaming parser      | No       | No event-stream input                      |
| Test generators       | No       | No SSE code under test here                |

## Version currency (of the audited checkout)

Tags: v0.7.0 (newest), v0.6.1, v0.6.0, v0.5.0, v0.4.0. HEAD = v0.7.0 plus
one test-only commit. Stable, stdlib-only, healthy — rejection is fit.

## Revisit trigger

Only if the tool ever consumes or serves event streams (e.g. live progress
streaming to a web UI). That implies the server/UI modes already rejected
in the httputil/cqrs-htmx reports.

## How to verify

```bash
rg -n 'go-sse|sseparse' --type go . go.mod go.sum   # absent
cd /home/lars/projects/go-sse && git tag --sort=-v:refname | head -5
cd /home/lars/projects/go-sse && ls */               # ssegen sseparse ssecycle
```
