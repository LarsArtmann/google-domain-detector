# google-domain-detector — AGENTS.md

Detect Google Workspace / GCP usage for a domain list from public DNS alone.

## Stack (details in STACK.md)

- Go 1.27, single module `github.com/larsartmann/google-domain-detector`
- CLI: **cmdguard v5** (`internal/cli`) — typed flag structs, batteries on
- Output: **go-output** (`internal/render`) — table/json/csv/tsv/markdown via
  sub-module blank imports that self-register renderers
- DNS: stdlib `net.Resolver` behind `resolve.Resolver` (interface); no
  third-party DNS library
- Event sourcing: **go-cqrs-lite** `system/v4` + `metaengine/v4`
  (`internal/scanstore`) — sqlite engine, journal + Map-query projections
- Ranges feed: `https://www.gstatic.com/ipranges/cloud.json`, disk cache
  (24 h TTL, stale-fallback), matching via `net/netip`

## Commands

```bash
go build ./...    # build
go test ./...     # tests (offline: fake resolvers, httptest feeds)
buildflow         # quality gate (lint/format/test/repair) — covered repo
nix build         # hermetic build (vendorHash in flake.nix; nix-hash-fix repairs)
go run ./cmd/google-domain-detector scan --no-ranges example.com
```

Event-sourced scans: `scan --event-store FILE.db` records every run as
events (one stream per domain, one per run); `replay --event-store FILE.db
[--run ID]` rebuilds the report from the journal alone, no DNS.

Manual go commands need the cache-env guard (stale inherited GOCACHE):
`export GOCACHE=$HOME/.cache/go-build-demo GOMODCACHE=$HOME/go/pkg/mod`.

## Architecture map

| Package              | Owns                                                                                                                                                            |
| -------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `internal/domain`    | Domain normalization/validation (URLs, ports, www-strip, punycode-in)                                                                                           |
| `internal/input`     | plain-list / CSV detection, header keyword match, dedup, Skipped reporting                                                                                      |
| `internal/classify`  | **all detection rules** — pure functions over record values; verdicts (Workspace certain/likely, GCP certain)                                                   |
| `internal/ranges`    | cloud.json parse/fetch/cache, CIDR membership (netip, 4-in-6 unmapped)                                                                                          |
| `internal/resolve`   | Resolver interface + PreferGo net.Resolver adapter                                                                                                              |
| `internal/scan`      | worker pool, per-domain timeout, NXDOMAIN short-circuit, DKIM probe only when MX+SPF silent; `Options.OnResult` streams finished results to sinks (event store) |
| `internal/render`    | go-output table formats + compact full-evidence JSON DTO                                                                                                        |
| `internal/scanstore` | event-sourced persistence via go-cqrs-lite `system.New` composition root: commands/events/deciders, metaengine projections, journal replay                      |
| `internal/cli`       | cmdguard wiring; `runScan` takes injected deps (test seam)                                                                                                      |

Rule changes go in `classify` (pure, table-tested). Probe orchestration
changes go in `scan`. Never put record-interpretation logic in `scan`.
Event-store schema changes go in `scanstore` (payloads are append-only
events — evolve, never rewrite).

## Library audit (2026-11-09)

All 22 LarsArtmann libs audited in `docs/review/libs/` (per-lib .md +
`index.html` dashboard). Durable verdicts: cmdguard 56/100 (unused: validate
tag, WithDoctorCheck, ExitError, env prefix) and go-output 31/100 (headline:
yaml/toml/jsonl registered but unreachable via the `--format` enum);
**go-atomic-write is the one adopt candidate** (S — replace hand-rolled
`writeCache`, use `WriteWithPerm`, never `WriteIfChanged`: it would skip
mtime refreshes the 24 h TTL depends on); go-error-family optional scoped to
`ranges` for exit codes; 18 absences structurally correct (server/UI/state
libs vs. one-shot CLI). Refresh when the dep set changes or a new
first-party lib is considered.

## Invariants (deliberate, do not "fix")

- Workspace verdict: MX (strong) → certain; SPF/DKIM (weak) → likely. Free
  Gmail "send as" produces SPF/DKIM without Workspace — never upgrade them.
- GCP IP matching uses **cloud.json only**, never goog.json: google.com
  itself must NOT flag as GCP by address.
- `isSubdomainOf` requires a label boundary — `evil.google.com.evil.example`
  must never match. Suffix-match tests pin this.
- CNAME self-targets (LookupCNAME returning the host itself) are not
  evidence; CNAME lookup errors are never domain errors.
- Lookup failures set `Result.Err` **only when the domain yielded zero
  evidence**; a partial failure with evidence is a normal result.
- Input parsing never fails the run for bad rows — skip + report count.

## Gotchas

- `net.ParseIP` returns 16-byte v4-in-6 form; `ranges.Set.Contains` unmaps
  (`addr.Is4In6() → Unmap()`) or v4 lookups silently check the v6 list.
- cmdguard battery commands (`version`, `doctor`, `describe`) print to the
  process stdout, NOT the injected writer — tests assert execution, not
  captured output, for them.
- go-output renderer registry: root module registers NOTHING; the blank
  imports in `internal/render` are load-bearing (drop one, lose a format).
- `encoding/json/v2` (migrated from v1 by go-auto-upgrade 2026-11-09 — jsonv2
  is stable in Go 1.27; render uses `jsontext.NewEncoder` for the compact
  one-line-per-domain JSON).
- `DomainObservedPayload` carries evidence only, never verdicts — verdicts
  are derived by `classify.Result` methods; duplicating them in events would
  be a split brain. Same rule for any new event payload.
- Observation streams are per-domain (stable stream ID = domain name), so
  whole-TLD scale never bloats one stream; run streams hold only
  started/completed. Replay keeps the LAST observation per domain per run.
- `replay` boots the system WITHOUT the projection host (`scanstore.Boot`)
  — journal replay must not pay for folding every read model first.
- Binary flag name is `--ranges-cache` (not `--cache-path`).
- Binary flag for the SQLite journal is `--event-store` (env `GDD_EVENT_STORE`).

## Git remotes

- `origin` → Forgejo `forgejo@evo-x2.home.lan:lars/google-domain-detector.git`
  (primary)
- `github` → `git@github.com:LarsArtmann/google-domain-detector.git` (mirror)

Push to both. The auto-commit daemon (pma) may commit mid-session — check
`git status` before committing; never assume.
