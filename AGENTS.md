# google-domain-detector — AGENTS.md

Detect Google Workspace / GCP usage for a domain list from public DNS alone.

## Stack (details in STACK.md)

- Go 1.27, single module `github.com/larsartmann/google-domain-detector`
- CLI: **cmdguard v5** (`internal/cli`) — typed flag structs, batteries on
- Output: **go-output** (`internal/render`) — table/json/csv/tsv/markdown via
  sub-module blank imports that self-register renderers
- DNS: stdlib `net.Resolver` behind `resolve.Resolver` (interface); no
  third-party DNS library
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

Manual go commands need the cache-env guard (stale inherited GOCACHE):
`export GOCACHE=$HOME/.cache/go-build-demo GOMODCACHE=$HOME/go/pkg/mod`.

## Architecture map

| Package             | Owns                                                                                                          |
| ------------------- | ------------------------------------------------------------------------------------------------------------- |
| `internal/domain`   | Domain normalization/validation (URLs, ports, www-strip, punycode-in)                                         |
| `internal/input`    | plain-list / CSV detection, header keyword match, dedup, Skipped reporting                                    |
| `internal/classify` | **all detection rules** — pure functions over record values; verdicts (Workspace certain/likely, GCP certain) |
| `internal/ranges`   | cloud.json parse/fetch/cache, CIDR membership (netip, 4-in-6 unmapped)                                        |
| `internal/resolve`  | Resolver interface + PreferGo net.Resolver adapter                                                            |
| `internal/scan`     | worker pool, per-domain timeout, NXDOMAIN short-circuit, DKIM probe only when MX+SPF silent                   |
| `internal/render`   | go-output table formats + compact full-evidence JSON DTO                                                      |
| `internal/cli`      | cmdguard wiring; `runScan` takes injected deps (test seam)                                                    |

Rule changes go in `classify` (pure, table-tested). Probe orchestration
changes go in `scan`. Never put record-interpretation logic in `scan`.

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
- Binary flag name is `--ranges-cache` (not `--cache-path`).

## Git remotes

- `origin` → Forgejo `forgejo@evo-x2.home.lan:lars/google-domain-detector.git`
  (primary)
- `github` → `git@github.com:LarsArtmann/google-domain-detector.git` (mirror)

Push to both. The auto-commit daemon (pma) may commit mid-session — check
`git status` before committing; never assume.
