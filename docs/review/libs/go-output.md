# go-output — utilization deep dive

> Reviewed: 2026-11-09 · Module: `github.com/larsartmann/go-output` (+ sub-modules)
> Status in google-domain-detector: **DIRECT, pinned v0.38.7** (root + `delimited`, `markdown`, `serialization`, `table`; six more sub-modules ride along indirectly: d2, daghtml, escape, graph, markup, plantuml, tree)
> Audited checkout: `/home/lars/projects/go-output` @ HEAD `ba50334` = **v0.38.7 + 7 docs/CI-only commits — pin is current, zero code drift**

## Verdict

**Adoption score: 31/100 (low) — 2 fully leveraged + 1 partial of 8
applicable capabilities.** The rendering path that is used is used
correctly, but the project registers **more formats than its own flag can
reach** and ignores two of three `RenderOptions` fields. The single
highest-value fix in this whole audit lives here: widening `--format` to
`yaml`/`toml`/`jsonl` costs one string (they are already blank-imported
and registered) — the registry is being paid for and left on the table.

## What the library is

Define data once (Table/Tree/Graph shapes), render to **16 formats**
(table, json, csv, tsv, markdown, xml, d2, yaml, html, tree, mermaid, dot,
jsonl, asciidoc, toml, plantuml) via a thread-safe self-registration
registry; heavy deps (yaml, toml, lipgloss, bubbletea) isolated in
sub-modules. Root package deps: only `golang.org/x/term` + go-branded-id.
17 sub-modules incl. `nom` (live inline progress), `tui` (bubbletea),
`escape`, `testhelpers`, `bdd`.

## Current usage (baseline, verified)

| Site                        | API used                                                                        |
| --------------------------- | -------------------------------------------------------------------------------- |
| `internal/render/render.go:14-18` | blank imports: delimited (csv,tsv), markdown, serialization (json,xml,yaml,jsonl,toml), table |
| `internal/render/render.go:72-83` | `NewTable` → `AddRow` → `RenderTable(data, format, RenderOptions{Writer: w})`    |
| `internal/cli/cli.go:114`   | `ParseFormat(f.Format)` — validates the `--format` flag                           |
| `internal/cli/cli.go:38`    | flag enum `values:"table,json,csv,tsv,markdown"` — **5 of 16 formats reachable** |
| `internal/cli/cli.go:146`   | `FormatJSON` special-case → custom DTO path (render.go:88-104), bypassing the registry |

## Capability inventory & grades

| # | Capability                                        | Grade             | Evidence / note                                                                    |
| - | ------------------------------------------------- | ----------------- | ---------------------------------------------------------------------------------- |
| 1 | Registry + blank-import registration pattern      | 🟢 Fully leveraged | render.go:14-18 — the load-bearing imports AGENTS.md warns about                    |
| 2 | RenderTable dispatch + RenderOptions.Writer       | 🟢 Fully leveraged | render.go:83 — clean dispatch for all table-shaped formats                           |
| 3 | Table construction API                            | 🟡 Partially used  | NewTable/AddRow used; `AddRowChecked` (fail-fast 5-col invariant) and `SetFooter` (summary row, rendered by 7 formats) unused |
| 4 | Format breadth via flag enum                      | 🔴 Missed (S)      | yaml/toml/jsonl registered but unreachable (flag enum blocks them); xml/html/asciidoc one blank import away (`markup` already in graph) |
| 5 | `RenderOptions.ColorMode` + `ParseColorMode`      | 🔴 Missed (S)      | a `--color=auto/always/never` flag; table renderer honors it; NO_COLOR/CI/FORCE_COLOR detection is free |
| 6 | `RenderOptions.Title`                             | 🔴 Missed (S)      | free `# title` on markdown, `<title>`+`<h1>` on html                                  |
| 7 | `RegisterUnknownRenderer` (DTO-in-registry)       | 🔴 Missed (S)      | would re-home the custom JSON DTO into the registry and remove the cli.go:146 special case (cosmetic) |
| 8 | `nom.InlineRenderer` live progress                | 🔴 Missed (M)      | genuinely fits a tens-of-thousands-domain batch: in-place ANSI activity tree, no alt-screen |
| — | Streaming writers (CSVWriter/JSONLWriter)         | ⚪ N/A             | results are materialized before render by design; streaming gains memory only, not correctness |
| — | Tree/Graph shapes (tree, dot, mermaid, d2, plantuml) | ⚪ N/A          | flat per-domain rows; a domain→evidence graph is a gimmick for this tool            |
| — | `tui` bubbletea progress                          | ⚪ N/A             | full-screen TUI is the wrong weight class for a pipe-friendly batch CLI             |
| — | `daghtml` interactive page                       | ⚪ N/A             | server artifact                                                                     |
| — | `escape` helpers                                 | ⚪ N/A             | markdown/CSV escaping already handled inside the renderers                           |
| — | WriteRendered helpers, MarshalJSONIndent          | ⚪ N/A             | equivalent one-liners exist; JSON path already streams via jsontext encoder         |

Score inputs: N=15 rows, A=8 applicable, Full=2, Partial=1 →
`round(100 × (2 + 0.5) / 8)` = **31 — low band**. Impact-weighted read:
the two Full rows are the load-bearing core (dispatch + registry), so
output correctness is not at risk; the score says "surface paid for,
surface unused".

## Top opportunities (impact × ease)

1. **Widen `values:` to `table,json,csv,tsv,markdown,yaml,toml,jsonl`** (S)
   — one string in cli.go:38; RenderTable already dispatches; yaml/toml
   consumers (spreadsheet-averse pipelines, config tooling) get output for
   free. **Best value/effort ratio in this audit.**
2. **`--color auto|always|never` flag** (S) — ParseColorMode +
   RenderOptions.ColorMode; correct piping behavior for free.
3. **`SetFooter` summary row** ("N domains · M workspace · K gcp") (S) —
   rendered by table/csv/tsv/markdown/html/xml, skipped by json/yaml/toml/
   jsonl — exactly the right semantic split.
4. **`AddRowChecked`** (S) — turns a silent column mismatch into an error.
5. **`RenderOptions.Title`** (S) — report header for markdown/html.
6. **nom progress** (M) — live per-worker activity during long scans;
   pulls lipgloss via the nom sub-module (already adjacent in graph).
7. **`RegisterUnknownRenderer(FormatJSON, dtoRenderer)`** (S, cosmetic) —
   dissolves the cli.go:146 special case; optional.

## Misuse check

- **Unreachable registered formats** (confirmed): serialization registers
  json/jsonl/toml/yaml table marshalers; the flag enum exposes none of
  yaml/toml/jsonl. `xml` parses (ParseFormat accepts all 16) but is not
  flag-reachable and its registrar (`markup`) is only an indirect dep —
  `--format xml` would need the blank import first.
- **FormatJSON bypass** (cli.go:146 → render.JSON): deliberate and
  documented (render.go:1-5 package comment — richer evidence DTO). Sound
  decision; the side effects (serialization's JSON table-marshaler is dead
  weight, Footer/Title never apply to JSON) are acceptable.

## Version currency

Pinned **v0.38.7 = newest tag** (666 tags; sub-modules ship in lockstep,
all v0.38.x). HEAD is v0.38.7 + docs/CI/website commits only. Fully
current; nothing to bump.

## How to verify

```bash
go run ./cmd/google-domain-detector scan --no-ranges --format yaml example.com   # rejected by flag enum today (gap #1)
rg -n 'RegisterTableMarshaler' $(go env GOMODCACHE)/github.com/larsartmann/go-output/serialization@v0.38.7/*.go  # json/jsonl/toml/yaml all registered
cd /home/lars/projects/go-output && git diff --stat v0.38.7..HEAD   # docs/website only
```
