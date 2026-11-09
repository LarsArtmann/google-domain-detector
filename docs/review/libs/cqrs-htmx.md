# cqrs-htmx — adoption & utilization review

> Reviewed: 2026-11-09 · Module: `github.com/larsartmann/cqrs-htmx/v4` (multi-module: root + adminui, auditlog, dashboardui, datastar, huma, health, identity-model, loginpage, setup, systemadapter, usermgmt[+oauth2/totp/webauthn], e2e)
> Status in google-domain-detector: **NOT IN MODULE GRAPH** (not adopted)
> Audited checkout: `/home/lars/projects/cqrs-htmx` @ `45a29b62` (2026-11-08) — newest tags `huma/v4.0.0`, root `v4.18.1`; HEAD 120+ commits past newest tag (co-developed lockstep with go-cqrs-lite via ~35 go.work replace pins)

## Verdict

**Do not adopt.** cqrs-htmx binds go-cqrs-lite commands/queries to
`net/http` + HTMX: declarative handler options for decoding, Casbin
authorization, validation, HTMX headers, partial rendering, SSE fan-out,
plus event-sourced user management (WebAuthn/OAuth2/TOTP) and ready-made
admin/dashboard/login UIs. Its README states the gatekeeper condition
outright: "**If you are not using go-cqrs-lite, this library is not for
you**" (README.md:77). The detector does not — and correctly does not (see
go-cqrs-lite.md) — use go-cqrs-lite.

## What the library is

One declarative line per endpoint: `app.Command(...)` with composable
`HandlerOption`s covering HTMX response headers (HX-Trigger/Push-Url/
Redirect), partial-vs-full rendering, CSRF, method override, conditional
GET/ETag, idempotency, SSE broadcaster with reconnect/Last-Event-ID,
OpenAPI collector, and a recommended middleware stack. Sub-modules add
event-sourced identity (usermgmt ~200+ files: sessions, roles, tenants,
lockout, audit log, SQL stores), adminui/dashboardui/loginpage UIs, an
all-in-one `setup`, a huma OpenAPI bridge, and an offline sync protocol.
Dependency weight: root go.mod = 18 direct requires (casbin, form decoding,
6 go-cqrs-lite modules, go-sse, httputil, go-etag, go-idempotency, ...) +
28 indirect; UI modules pull templ + templ-components transitively.

## Current usage in google-domain-detector

None — not imported, not in the graph.

## Why we should NOT use it

1. **Absolute process/UI mismatch.** Every capability presumes an inbound
   HTTP server, browser interaction, cookie sessions, and an event-sourced
   backend. The detector is headless: no listener, no UI, no auth, no
   writes.
2. **Self-declared incompatibility.** The library's own README gatekeeper
   sentence disqualifies any consumer not on go-cqrs-lite.
3. **Dependency explosion for a hermetic nix single-binary CLI:** casbin,
   go-sse, go-etag, go-idempotency, templ... for zero used features.

## Capability ↔ project applicability

| Capability                            | Applies? | Reason                                        |
| ------------------------------------- | -------- | --------------------------------------------- |
| Command/query → HTTP handler builder  | No       | No commands, no handlers                       |
| HTMX partial rendering / headers      | No       | No browser client                              |
| usermgmt (WebAuthn/OAuth2/TOTP)       | No       | No users, no auth                              |
| adminui / dashboardui / loginpage     | No       | No UI                                          |
| SSE broadcaster                       | No       | No event streams                               |
| Conditional GET / ETag helpers        | No       | Server-response concerns (see go-etag.md)      |
| Offline sync protocol                 | No       | No state to sync                               |

## Version currency (of the audited checkout)

Tags by date: `huma/v4.0.0`, `loginpage/v4.17.0`, root `v4.18.1`,
`v4.18.0`, `systemadapter/v4.13.0`. HEAD is 120–127 commits past the newest
tags — very active. Rejection is fit, not quality.

## Revisit trigger

None as this product. Only a ground-up rewrite as a server-rendered app on
go-cqrs-lite would change the verdict; both rejections stack.

## How to verify

```bash
rg -n 'cqrs-htmx' --type go . go.mod go.sum   # absent
cd /home/lars/projects/cqrs-htmx && git tag --sort=-creatordate | head -5
cd /home/lars/projects/cqrs-htmx && rg -n 'not for you' README.md
```
