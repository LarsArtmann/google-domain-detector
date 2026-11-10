# STACK — google-domain-detector

> The deliberate foundation of this project: every significant dependency was
> chosen, not accumulated. Generated 2026-11-09 by the stack-selection skill
> (profile: Go CLI tool). This is a LIVING document — update the row when the
> reason changes, delete the row when the dependency goes.
> Enforced by `stack-selection/scripts/stack-drift-check.sh <project-dir>`:
> every direct github.com/larsartmann/* require must have a Chosen foundation
> row. Exit 0 = consistent; exit 1 = undeclared or contradicted dep; exit 2 =
> this file is missing.

## Profile

| Axis         | Answer                                                                                              |
| ------------ | --------------------------------------------------------------------------------------------------- |
| Shape        | CLI (one-shot batch scanner; optionally event-sourced)                                              |
| Domain       | DNS technographics — Google Workspace / GCP evidence for a domain list                              |
| Persistence  | optional: scan runs as events in a SQLite file (`--event-store`); plus the cached GCP IP-range feed |
| Interfaces   | terminal (args/stdin/CSV in; table/JSON/CSV/TSV/Markdown out)                                       |
| Scale target | single user, hundreds–tens of thousands of domains per run                                          |
| Constraints  | single binary, nix build, public deps only, tests offline (fake DNS)                                |

## Chosen foundation

| Category         | Choice (first-party = ★)                                    | Why this, not that                                                                                                                                                                                                                                                 | Owning skill / docs                       | Revisit when                                            |
| ---------------- | ----------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ | ----------------------------------------- | ------------------------------------------------------- |
| CLI framework    | ★ cmdguard v5                                               | typed struct-tag flags (enum `values`, Duration, env), builder wiring, version/doctor/describe batteries                                                                                                                                                           | `cmdguard` skill                          | command set grows beyond `scan`                         |
| Output rendering | ★ go-output                                                 | one `Table` → table/json/csv/tsv/markdown via registry dispatch; format-enum flag is free                                                                                                                                                                          | README + pkg.go.dev                       | output needs exceed its formats                         |
| DNS lookups      | stdlib `net.Resolver`                                       | MX/TXT/CNAME/IP/NS are all stdlib calls; rejected `miekg/dns` — no raw-packet or EDNS need, UDP internals                                                                                                                                                          | stdlib docs                               | need for specific EDNS/DNSSEC options arises            |
| Concurrency      | stdlib worker pool                                          | one fan-out loop with channels; rejected `golang.org/x/sync/errgroup` — nothing else needs it, zero-dep                                                                                                                                                            | —                                         | structured per-lookup cancellation needed               |
| JSON             | stdlib `encoding/json/v2`                                   | Go 1.27 ships jsonv2 stable — the hermetic nix build (GOTOOLCHAIN=local) compiles it, no GOEXPERIMENT plumbing; rejected v1 — superseded per how-to-golang. Written on v1, migrated by go-auto-upgrade 2026-11-09                                                  | how-to-golang                             | —                                                       |
| Nix packaging    | go-nix-helpers                                              | `flakeModules.go-standard` is the fleet pattern (vendorHash, goPkgAttr, devshell); rejected hand-rolled `buildGoModule`                                                                                                                                            | `nix-private-go-repos` (public deps only) | —                                                       |
| Quality pipeline | BuildFlow                                                   | lint/format/test/repair loop + pre-commit; this repo ships only a minimal `.buildflow.yml`                                                                                                                                                                         | `buildflow` skill                         | —                                                       |
| Event sourcing   | ★ go-cqrs-lite `system/v4` + `metaengine/v4` (sqliteengine) | owner directive 2026-11-09: whole-TLD runs must be durable, replayable, and keep per-domain history. Composition root (`system.New`) + Map-query projections per ADR-0123; rejected hand-rolled SQLite journal — split brain vs. the fleet's one storage substrate | `go-cqrs-lite` skill                      | read models outgrow point lookups (need filtered scans) |

Rules for this table:

- One row per concern actually needed by the profile — not per category in
  the capability map. Absent concern = absent row.
- "Why this, not that" must name the rejected alternative. A choice without
  a rejected alternative was not a decision.
- First-party libs are marked ★ and MUST have been verified to exist
  (README + go.mod + tagged version) before this table was written.

## Deliberately excluded

| Candidate                    | Reason                                                                               |
| ---------------------------- | ------------------------------------------------------------------------------------ |
| go-retry                     | `net.Resolver` retries internally; a batch tool re-runs cheaply                      |
| go-composable-business-types | no Money/Email/DataPoint semantics in this domain                                    |
| go-branded-id                | scanned domains are input values, not entity IDs; a validated `Domain` type suffices |
| samber/do                    | no service graph beyond cmdguard's own DI                                            |
| go-ndjson                    | input is CSV/plain lines, not newline-delimited JSON                                 |
| project-discovery-sdk        | scans arbitrary remote domains, not local project directories                        |
| miekg/dns                    | raw packet access unneeded; stdlib resolver covers every record type probed          |

Every high-importance candidate the profile plausibly touched belongs here
with a reason — silence about a known candidate is an unmade decision, not a
decision.

## Wiring order

1. `go mod init` + `go get github.com/larsartmann/cmdguard/v5@v5.0.0` + go-output (root + `table`, `delimited`, `serialization`, `markdown` sub-modules)
2. cmdguard wiring per the `cmdguard` skill (Builder, per-command F-struct, batteries on)
3. flake.nix via go-nix-helpers `go-standard`; public deps only — no `nix-private-go-repos` machinery needed
4. Quality gate: BuildFlow (`buildflow` after `.buildflow.yml` marker; `buildflow precommit install`)

## Decision log

- 2026-11-09 — Initial selection (stack-selection skill, session: "detect Google usage from domains" greenfield build).
- 2026-11-09 — JSON row updated: go-auto-upgrade migrated `encoding/json` → `encoding/json/v2` once the Go 1.27 toolchain made it stable; the initial v1 choice (GOEXPERIMENT concern) is obsolete.
- 2026-11-09 — go-cqrs-lite moved from "deliberately excluded" (YAGNI for a read-only batch scan) to the chosen foundation: owner directive to event-source scans ahead of whole-TLD runs. The original exclusion assumed a one-shot tool; run history, crash-safe journals, and replay changed the profile. Integration is `system.New` + `metaengine` only (ADR-0123), behind `--event-store` — the no-store path stays dependency-free at runtime.
- 2026-11-10 — `--format` kept; cmdguard `WithOutputFormat`/`-o` rejected. The `-o` flag is GLOBAL (one format for every subcommand) and its resolved value is read through the CLI object at handler time, which would break the per-command flag structs and the injected-deps test seam (`runScan(f ScanFlags, …)`). `-o` also reads as "output file" in unix convention; `--format` says what it does. Registry-derived enum drift-proofing stays on `--format` instead (plan T22). Decided before the v0.1.0 tag so a later flip is a versioned breaking change, never a silent one.
