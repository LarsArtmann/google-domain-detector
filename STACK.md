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

| Axis         | Answer                                                                 |
| ------------ | ---------------------------------------------------------------------- |
| Shape        | CLI (one-shot batch scanner)                                           |
| Domain       | DNS technographics — Google Workspace / GCP evidence for a domain list |
| Persistence  | none (one cached artifact: the GCP IP-range feed on disk)              |
| Interfaces   | terminal (args/stdin/CSV in; table/JSON/CSV/TSV/Markdown out)          |
| Scale target | single user, hundreds–tens of thousands of domains per run             |
| Constraints  | single binary, nix build, public deps only, tests offline (fake DNS)   |

## Chosen foundation

| Category         | Choice (first-party = ★) | Why this, not that                                                                                         | Owning skill / docs      | Revisit when                                          |
| ---------------- | ------------------------ | ---------------------------------------------------------------------------------------------------------- | ------------------------ | ----------------------------------------------------- |
| CLI framework    | ★ cmdguard v5            | typed struct-tag flags (enum `values`, Duration, env), builder wiring, version/doctor/describe batteries   | `cmdguard` skill         | command set grows beyond `scan`                       |
| Output rendering | ★ go-output              | one `Table` → table/json/csv/tsv/markdown via registry dispatch; format-enum flag is free                  | README + pkg.go.dev      | output needs exceed its formats                       |
| DNS lookups      | stdlib `net.Resolver`    | MX/TXT/CNAME/IP/NS are all stdlib calls; rejected `miekg/dns` — no raw-packet or EDNS need, UDP internals  | stdlib docs              | need for specific EDNS/DNSSEC options arises          |
| Concurrency      | stdlib worker pool       | one fan-out loop with channels; rejected `golang.org/x/sync/errgroup` — nothing else needs it, zero-dep    | —                        | structured per-lookup cancellation needed             |
| JSON             | stdlib `encoding/json`   | rejected `encoding/json/v2` — still GOEXPERIMENT-gated, and the hermetic nix build (GOTOOLCHAIN=local) would need env plumbing | how-to-golang        | jsonv2 is non-experimental in the fleet toolchain     |
| Nix packaging    | go-nix-helpers           | `flakeModules.go-standard` is the fleet pattern (vendorHash, goPkgAttr, devshell); rejected hand-rolled `buildGoModule` | `nix-private-go-repos` (public deps only) | — |
| Quality pipeline | BuildFlow                | lint/format/test/repair loop + pre-commit; this repo ships only a minimal `.buildflow.yml`                  | `buildflow` skill        | —                                                     |

Rules for this table:

- One row per concern actually needed by the profile — not per category in
  the capability map. Absent concern = absent row.
- "Why this, not that" must name the rejected alternative. A choice without
  a rejected alternative was not a decision.
- First-party libs are marked ★ and MUST have been verified to exist
  (README + go.mod + tagged version) before this table was written.

## Deliberately excluded

| Candidate                    | Reason                                                                              |
| ---------------------------- | ----------------------------------------------------------------------------------- |
| go-cqrs-lite                 | no events or invariants — a batch read-only scan (YAGNI)                            |
| go-retry                     | `net.Resolver` retries internally; a batch tool re-runs cheaply                     |
| go-composable-business-types | no Money/Email/DataPoint semantics in this domain                                   |
| go-branded-id                | scanned domains are input values, not entity IDs; a validated `Domain` type suffices |
| samber/do                    | no service graph beyond cmdguard's own DI                                           |
| go-ndjson                    | input is CSV/plain lines, not newline-delimited JSON                                |
| project-discovery-sdk        | scans arbitrary remote domains, not local project directories                      |
| miekg/dns                    | raw packet access unneeded; stdlib resolver covers every record type probed         |
| encoding/json/v2             | GOEXPERIMENT plumbing vs. hermetic nix build (see chosen table)                     |

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
