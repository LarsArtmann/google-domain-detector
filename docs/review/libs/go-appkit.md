# go-appkit — adoption & utilization review

> Reviewed: 2026-11-09 · Module: `github.com/larsartmann/go-appkit` (multi-module: core + config, cqrs, docs, errorpages, flightrecorder(+health), health, httpx, otel, preset, realtime, security, systemd)
> Status in google-domain-detector: **NOT IN MODULE GRAPH** (not adopted)
> Audited checkout: `/home/lars/projects/go-appkit` @ `86bd621` (2026-11-08) — newest tags `httpx/v0.2.0`, `cqrs/v0.9.0`, root `v0.12.0`; HEAD 10 commits past `httpx/v0.2.0`
> Consumer repo state at review: google-domain-detector @d07a343 (2026-11-09) — verdicts are point-in-time; re-verify citations against HEAD before relying on them

## Verdict

**Do not adopt.** go-appkit is a production HTTP-service lifecycle kit: one
`NewService(ServiceConfig{Addr})` gives a long-running server with graceful
drain, a middleware chain, health endpoints, structured logging, and error
classification. google-domain-detector binds no port, serves nothing, and
exits after rendering. Every module presumes the server lifecycle; the
single non-server module (`config`) duplicates flag handling the project
already gets from cmdguard/v5.

## What the library is

Verified against README.md and module sources:

- **core** — `NewService` lifecycle: SIGINT/SIGTERM drain+shutdown, start/
  drain/shutdown hooks, default middleware chain (ServerTiming → Recovery
  → RequestID → Logging → Timeout → SecurityHeaders), opt-in zero-dep
  Prometheus `/metrics`, `/version`, and a `testkit` that boots the real
  service for tests.
- **config** — env-first struct-tag config loader (defaults/YAML/JSON/env/
  flag in one `Load`); explicitly "NOT a config framework" and explicitly
  defers to cmdguard as the flag engine.
- **cqrs** — lifecycle-managed `EventService` wrapping go-cqrs-lite's
  system root, with DLQ and typed command/query facade.
- **httpx** — handler DX battery: `(T, error)` returns → correct HTTP
  responses, JSON serialization, classification-aware error contracts,
  bind+validate.
- **realtime** — SSE hub + handler (broadcast, replay, heartbeat,
  Last-Event-ID resume). **health** — go-health probes + live dashboard.
  **flightrecorder(+health)** — on-demand runtime/trace capture.
  **otel** — opt-in tracing/metrics. **security** — CSRF, rate limiting,
  body limits, CSP nonce. **systemd** — sd_notify Type=notify.
  **errorpages** — pretty HTML/JSON error pages. **docs** — generated
  OpenAPI/AsyncAPI/D2. **preset** — one-call composition of the above.
- Core deps: charmbracelet/log, go-error-family, httputil (+server_timing).

## Current usage in google-domain-detector

None — not imported, not in the graph.

## Why we should NOT use it

1. **Total problem-class mismatch.** Graceful drain windows, readiness
   probes, request middleware, per-request security — there is nothing to
   attach them to in a one-shot CLI.
2. **The only CLI-adjacent module is redundant.** `config/` is the single
   non-server module, but the detector's flag/env needs are met by
   cmdguard/v5 — and appkit's config module itself treats cmdguard as the
   flag engine, so adopting it would layer a YAML/env config tier onto a
   single-user tool that has no config file format, for zero benefit.
3. **Overlaps are already covered lighter:** error classification is
   go-error-family (already in this project's graph via cmdguard), output
   is go-output (direct), flags are cmdguard (direct). appkit has no
   terminal-output helpers at all.
4. Go floor 1.27.3 vs this project's `go 1.27` — an unnecessary bump if it
   were adopted.

## Capability ↔ project applicability

| Module                                             | Applies? | Reason                                            |
| -------------------------------------------------- | -------- | ------------------------------------------------- |
| core Service                                       | No       | No listener, no drain/shutdown semantics          |
| config                                             | No       | Redundant with cmdguard flags/env (its own words) |
| httpx                                              | No       | Inbound-handler DX                                |
| realtime/SSE                                       | No       | No event streams                                  |
| health(+dash)                                      | No       | No probing process                                |
| security                                           | No       | Browser/request concerns                          |
| systemd                                            | No       | Not run under systemd                             |
| otel / docs / errorpages / preset / flightrecorder | No       | Server observability stack                        |

## Version currency (of the audited checkout)

Tags by date: `httpx/v0.2.0` (2026-11-08), `cqrs/v0.9.0`, `realtime/v0.2.0`,
`systemd/v0.2.0`, `httpx/v0.1.0`; root latest `v0.12.0`. HEAD 10 commits
past the newest tag — actively maintained. Rejection is fit, not quality.

## Revisit trigger

Only if the tool ever ships as an HTTP service (e.g. a hosted batch-API
mode). That would be a new product profile per STACK.md.

## How to verify

```bash
rg -n 'appkit' --type go . go.mod go.sum    # absent
cd /home/lars/projects/go-appkit && git tag --sort=-creatordate | head -5
cd /home/lars/projects/go-appkit && rg -n 'cmdguard' config/README.md    # config defers to cmdguard
```
