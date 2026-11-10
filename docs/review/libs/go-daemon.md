# go-daemon — adoption & utilization review

> Reviewed: 2026-11-09 · Module: `github.com/LarsArtmann/go-daemon` (capital L)
> Status in google-domain-detector: **NOT IN MODULE GRAPH** (not adopted)
> Audited checkout: `/home/lars/projects/go-daemon` @ `e8e1a1f` = v0.4.0 + 32 commits (code-relevant: injectable JSON/CBOR codec negotiation; rest is chores/docs)
> Consumer repo state at review: google-domain-detector @d07a343 (2026-11-09) — verdicts are point-in-time; re-verify citations against HEAD before relying on them

## Verdict

**Do not adopt.** Despite the name, go-daemon is not an OS daemonizer — it
is building blocks for **long-lived HTTP daemons serving over unix domain
sockets**: socket-server lifecycle with sd_notify readiness, JSON/CBOR
content negotiation, WHATWG SSE parsing, and an execution-trace flight
recorder. It contains no fork, no pidfile, no setsid, no signal handling
(verified: zero non-test matches for
`pidfile|fork|setsid|os.Signal|SIGTERM`). google-domain-detector has no
listener, no socket, no systemd unit, and exits when the scan finishes.

## What the library is

Verified in socket.go, notify.go, negotiation.go, sse.go,
flightrecorder.go, transport.go, write.go:

- `NewSocketServer(path, opts)` — stale-socket cleanup, 0600 socket perms,
  socket removal on every exit path (socket.go:183); `NewServer(network,
  addr)` for TCP; ctx-driven graceful shutdown with grace period and
  ordered startup/shutdown hooks (socket.go:313-335).
- systemd integration (no-ops without systemd): `NotifyReady`,
  `NotifyStopping`, `Watchdog` keepalive at WatchdogSec/2, notify counters
  (notify.go:46-102). Supervision (fork/pidfiles) is an explicit non-goal
  (README.md:101-108; docs/supervision.md covers unit files instead).
- Content negotiation: `ResolveRequestCodec`/`ResolveResponseCodec`,
  injectable `WithJSONCodec`/`WithCBORCodec`, `JSONv2Codec`
  (negotiation.go:48-231, jsonv2.go:30).
- Clients: `NewHTTPClient`/`NewHTTPClientNoTimeout` over unix sockets
  (transport.go:25-41).
- SSE: `ParseSSEData` WHATWG-conformant framing (sse.go:42).
- FlightRecorder: runtime trace capture with slow-request auto-trigger,
  Prometheus metrics, streaming `TraceHandler` (flightrecorder.go:198-380).

Deps: coreos/go-systemd/v22, go-codec, go-flightrecorder, go-sse/sseparse,
prometheus/client_golang (+10 indirect).

## Current usage in google-domain-detector

None — not imported, not in the graph.

## Why we should NOT use it

1. **Everything presupposes a listener.** Readiness notifications, watchdog
   keepalives, socket perms, shutdown drain — inapplicable to a one-shot
   CLI that exits after rendering.
2. **It would not even daemonize the tool if we wanted that** — supervision
   is a stated non-goal; the name promises fork/pidfile machinery the code
   deliberately lacks.
3. **Dependency weight:** prometheus/client_golang + go-systemd + SSE +
   codec stacks dragged into a hermetic nix single-binary build for zero
   used surface.

## Capability ↔ project applicability

| Capability                          | Applies? | Reason                            |
| ----------------------------------- | -------- | --------------------------------- |
| Unix-socket HTTP server lifecycle   | No       | No listener, no socket            |
| sd_notify readiness / watchdog      | No       | Not run under systemd TYPE=notify |
| JSON/CBOR negotiation, WriteEncoded | No       | No requests to negotiate          |
| SSE parsing                         | No       | No event streams                  |
| FlightRecorder trace streaming      | No       | No long-lived process to diagnose |
| Graceful drain hooks                | No       | Process lifetime is a single scan |

## Version currency (of the audited checkout)

Tags: v0.4.0 (newest), v0.3.0, v0.2.0, v0.1.1, v0.1.0. HEAD is 32 commits
past v0.4.0. Healthy and active; rejection is fit, not quality.

## Revisit trigger

None plausible. If a daemon mode were ever wanted it would be a different
product; even then this library serves _socket-HTTP daemons_, not
process daemonization.

## How to verify

```bash
rg -n 'go-daemon' --type go . go.mod go.sum   # absent
cd /home/lars/projects/go-daemon && git tag --sort=-v:refname | head -5
cd /home/lars/projects/go-daemon && rg -n 'pidfile|setsid|fork' --type go -g '!*_test.go'   # zero hits
```
