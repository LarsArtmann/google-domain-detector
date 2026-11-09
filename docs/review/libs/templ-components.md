# templ-components — adoption & utilization review

> Reviewed: 2026-11-09 · Module: `github.com/larsartmann/templ-components` (multi-module: root + themed, ui, base, charts/*, go-htmx, demo-site, devserver, docs)
> Status in google-domain-detector: **NOT IN MODULE GRAPH** (not adopted)
> Audited checkout: `/home/lars/projects/templ-components` @ `5c0d170a` = `charts/echarts/v1.27.0` + 87 commits (v1.27.0 family is the latest tag set across modules)

## Verdict

**Do not adopt.** templ-components is a Go UI component library: 180+
`templ`-generated HTML components (inputs, tables, modals, drawers,
tooltips, charts incl. ECharts, typography, buttons, feedback) styled with
Tailwind v4, wired for HTMX/DataStar interactions, served by an HTTP
server, with a demo-site and live devserver. The detector renders to a
terminal via go-output and has no HTML surface. If an HTML report format
was ever wanted, go-output's `daghtml` sub-module (already in the module
graph) is the correct tool, not a component library that drags in Tailwind,
templ codegen, and an HTTP mux.

## What the library is

Component families with typed enum props (ButtonVariant, InputType,
TooltipPlacement, ...), theme tokens (light/dark, CSS custom properties),
icon paths (~250 inline SVGs), HTMX helpers (indicators, boosted forms),
ECharts/chart.js bindings with headless render support, and a demo site
for browsing them all. Verified against the checkout's module layout and
the templ-components skill's component inventory.

## Current usage in google-domain-detector

None — not imported, not in the graph (templ itself appears only as an
indirect dep of go-output's tui).

## Why we should NOT use it

1. **No UI, no server, no HTML.** Every component compiles to templ HTML
   for browser delivery.
2. Toolchain mismatch: adopting means templ codegen + Tailwind scanning +
   asset serving in a single-binary terminal tool.
3. The only adjacent idea — "self-contained HTML report" — is covered by
   go-output's daghtml (a static D3 artifact) without any of that weight.

## Capability ↔ project applicability

| Capability            | Applies? | Reason                                       |
| --------------------- | -------- | -------------------------------------------- |
| 180+ HTML components  | No       | No HTML output                               |
| Charts (ECharts)      | No       | No charts, no browser                        |
| HTMX/DataStar helpers | No       | No interactivity surface                     |
| Themes/icons          | No       | Terminal styling is lipgloss/fang (cmdguard) |

## Version currency (of the audited checkout)

Latest tag family: v1.27.0 (root, base, themed, ui + charts sub-modules).
HEAD is 87 commits past `charts/echarts/v1.27.0` — very active. Rejection
is fit, not quality.

## Revisit trigger

Only for a browser-facing product (web dashboard/report viewer) — already
ruled out across go-datastar/cqrs-htmx/go-appkit reports.

## How to verify

```bash
rg -n 'templ-components' --type go . go.mod go.sum    # absent
cd /home/lars/projects/templ-components && git tag --sort=-creatordate | head -5
ls /home/lars/projects/templ-components                # component modules + demo-site + devserver
```
