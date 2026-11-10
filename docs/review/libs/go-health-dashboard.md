# go-health-dashboard — adoption & utilization review

> Reviewed: 2026-11-09 · Module: `github.com/larsartmann/go-health-dashboard`
> Status in google-domain-detector: **NOT IN MODULE GRAPH** (not adopted)
> Audited checkout: `/home/lars/projects/go-health-dashboard` @ `d15fcb6` = v0.13.2 + 14 commits (villain-theme UI, check-name uniqueness, NixOS module, Redis sub-flake — all unreleased)
> Consumer repo state at review: google-domain-detector @f5ea3ac (2026-11-09) — verdicts are point-in-time; re-verify citations against HEAD before relying on them

## Verdict

**Do not adopt.** go-health-dashboard is a **server-rendered HTML status
page** for the go-health library: it mounts `/health/dashboard` on an
`http.ServeMux`, renders a themed card per go-health check with live
values, auto-refresh, Kubernetes pod rollups, and Redis exporters. It is
100% downstream of a running HTTP service with health checks.
google-domain-detector is a terminal CLI with no server, no go-health
registry (see go-health.md), and no page to render. There is no scenario,
current or plausible, where a browser observes this tool's health.

## What the library is

Verified in the checkout: `Dashboard` struct with themed layout (Amsterdam/
Villain options), `AsHTML`/`AsJSON` renderers, `MountDashboard(mux, ...)`
wiring, per-check cards, live-variable sections, auto-refresh meta tags,
Kubernetes pod-status rollup, Redis info exporters, background refresher
goroutine. Depends on go-health (peer modules) and renders into responses.

## Current usage in google-domain-detector

None — not imported, not in the graph.

## Why we should NOT use it

1. Requires everything go-health requires (long-lived service) **plus** an
   HTTP mux and a browser — a double dependency on the rejected server
   model.
2. Terminal output is the product surface here; a dashboard for a
   sub-second process is category confusion.

## Capability ↔ project applicability

| Capability              | Applies? | Reason               |
| ----------------------- | -------- | -------------------- |
| HTML dashboard mounting | No       | No mux, no server    |
| Check cards / live view | No       | No health checks     |
| K8s/Redis exporters     | No       | No cluster, no Redis |

## Version currency (of the audited checkout)

Tags: v0.13.2 (newest), v0.13.1, v0.13.0, v0.12.x, v0.11.x. HEAD is 14
commits past v0.13.2 — actively maintained. Rejection is fit, not quality.

## Revisit trigger

None independently; inherits go-health's (already-rejected) trigger only.

## How to verify

```bash
rg -n 'health-dashboard' --type go . go.mod go.sum    # absent
cd /home/lars/projects/go-health-dashboard && git tag --sort=-v:refname | head -5
cd /home/lars/projects/go-health-dashboard && rg -n 'func MountDashboard' *.go
```
