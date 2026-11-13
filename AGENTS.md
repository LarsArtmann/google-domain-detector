# google-domain-detector — AGENTS.md

Detect Google Workspace / GCP usage for a domain list from public DNS alone.

## Stack (details in STACK.md)

- Go 1.27, single module `github.com/larsartmann/google-domain-detector`
- CLI: **cmdguard v5** (`internal/cli`) — typed flag structs, batteries on
- Output: **go-output** (`internal/render`) — blank imports self-register
  renderers; DNS: stdlib `net.Resolver` behind `resolve.Resolver`
- Event sourcing: **go-cqrs-lite** `system/v4` + `metaengine/v4`
  (`internal/scanstore`) — sqlite journal + Map-query projections
- Ranges: gstatic cloud.json, 24 h disk cache, `net/netip` matching

## Commands

```bash
go build ./...    # build
go test ./...     # tests (offline: fake resolvers, httptest feeds)
buildflow         # quality gate (lint/format/test/repair) — covered repo
nix build         # hermetic build (vendorHash in flake.nix; nix-hash-fix repairs)
go run ./cmd/google-domain-detector scan --no-ranges example.com
```

Event-sourced scans: `scan --event-store FILE.db` journals every run;
`replay --event-store FILE.db [--run ID]` rebuilds the report from the
journal alone, no DNS. Manual go commands need the cache-env guard:
`export GOCACHE=$HOME/.cache/go-build-demo GOMODCACHE=$HOME/go/pkg/mod`.

## Architecture map

| Package              | Owns                                                                                                                                                                                                                       |
| -------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `internal/domain`    | Domain normalization/validation (URLs, ports, www-strip, punycode-in)                                                                                                                                                      |
| `internal/input`     | plain-list / CSV detection, header keyword match, dedup, Skipped reporting                                                                                                                                                 |
| `internal/classify`  | **all detection rules** — pure functions over record values; verdicts (Workspace certain/likely, GCP certain)                                                                                                              |
| `internal/ranges`    | cloud.json parse/fetch/cache, CIDR membership (netip, 4-in-6 unmapped)                                                                                                                                                     |
| `internal/resolve`   | Resolver interface + PreferGo net.Resolver adapter                                                                                                                                                                         |
| `internal/scan`      | worker pool, per-domain timeout, NXDOMAIN short-circuit, DKIM probe only when MX+SPF silent; `Options.OnResult` streams finished results to sinks (event store)                                                            |
| `internal/render`    | go-output table formats + compact full-evidence JSON DTO                                                                                                                                                                   |
| `internal/scanstore` | event-sourced persistence via go-cqrs-lite `system.New` composition root: commands/events/deciders, metaengine projections, journal replay, verdict-flip delta (FlipDiff — errored lookups are unknowns, never departures) |
| `internal/cli`       | cmdguard wiring; `runScan` takes injected deps (test seam); `domains.go` = streaming chunked input feeder; `show.go` = read-model queries                                                                                  |

Rule changes go in `classify` (pure, table-tested). Probe orchestration
changes go in `scan`. Never put record-interpretation logic in `scan`.
Event-store schema changes go in `scanstore` (payloads are append-only
events — evolve, never rewrite).

## Library audit (2026-11-09, point-in-time — re-verify vs go.mod)

All 22 LarsArtmann libs audited in `docs/review/libs/` (per-lib .md +
`index.html` dashboard; `scripts/audit-freshness.sh` + the CI
lib-freshness job flag drift). Durable verdicts: cmdguard 56/100 and
go-output 31/100 (most gaps since closed — format enum widened, colors,
SetFooter, atomic-write adopted; re-audit before acting). Refresh when the
dep set changes or a new first-party lib is considered.

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

- **Go's resolver conflates NXDOMAIN and NODATA into one `IsNotFound`
  error** ("no such host" for both; Go 1.27 constructs `errNoSuchHost` for
  each) — reading it as "the domain is dead" stamped ~90% of wave A's
  22,010 NX verdicts false. Name existence MUST go through
  `resolve.LookupExistence` (rcode oracle); below an Exists verdict,
  sub-lookup IsNotFound is ordinary record absence (`isRecordAbsence`).
- **NEVER edit a bash script while a detached instance is mid-run** — bash
  reads by byte offset; an in-place edit shifts the running instance's
  pending read into garbage (the 2026-11-13 nxfix wedge; the edit/write
  tools rewrite the SAME inode — mid-wave-B near-miss caught via
  /proc/<pid>/fd/255). Replace via a NEW inode (`install -m 755 /tmp/new
  old`). Wedge recovery: drain the scan (SIGINT via `pkill` — `kill` is a
  no-op builtin here), read run1.stderr for the run id, write `run-id`,
  re-run the gate with `--manifest`, snapshot, log in monitor.log.
- **DNS-over-TCP cannot be forced via Go's Dial callback** — overriding
  udp→tcp in Dial sends unprefixed data. TCP needs a hand-rolled wire
  client (the existence oracle) or the TCP-upstream unbound variant
  (`scripts/unbound-tcp.conf`, client hop stays UDP).
- **Failed-lookup error strings lie about the port**: DNSError.Server shows
  the resolv.conf address even when Dial sent the query to `--resolver`
  (`internal/resolve` discards Go's address) — never diagnose "the scan
  used the system resolver" from the error string.
- **metaengine dispatches ONE fold per (query, event type) — last
  registered wins** (`foldByEvent`, metaengine v4.23.0). `views.go`
  registers insert- AND update-shaped folds for `scan.domain_observed`;
  only the update fold runs (upsert — pinned, `Observations == 1`). The
  insert fold is NOT dead weight: it declares the Map keyType
  `QueryKeyType()` reads. Never "simplify" it away without the
  read-model round-trip tests on memory AND sqlite.
- **G4 journal capacity + sink/cancel semantics moved to census.md**
  (§"G4 journal capacity": 100k-event numbers, sink 3× retry + dropped
  count, Ctrl-C drain contract, negative-cache persistence lesson).
- **Host-level outbound UDP:53 outages happen** (2026-11-12: TCP:53 kept
  working; ~3 450 garbage observations). `census-wave.sh` pre-flights real
  upstreams (exit 5; a cache-answering forwarder proves nothing);
  TCP-forwarder waves set `GDD_PREFLIGHT_TRANSPORT=tcp`. This host also
  runs a live resolver on `[::1]:53`.
- cmdguard's flag-binding layer returns `fmt.Errorf`-wrapped SENTINELS,
  not `*v5.EnumError`/`*ExitError` structs — `errors.As` for the structs
  finds nothing. `internal/cli.Run` maps that sentinel family to usageError
  (exit 2) via `isFlagValueError`; new flag-level usage classes join it.
- Library sharp edges: `net.ParseIP` returns 16-byte v4-in-6 form
  (`ranges.Set.Contains` unmaps, or v4 lookups silently check the v6
  list); cmdguard battery commands (`version`, `doctor`, `describe`) print
  to process stdout, not the injected writer; go-output's root module
  registers NOTHING (the blank imports in `internal/render` are
  load-bearing); `encoding/json/v2`'s jsontext ERRORS on invalid UTF-8
  strings where v1 coerced (see the hostile-TXT gotcha).
- **jq on this host is a SHIM**: breaks on relative paths, no auto-iterate
  (explicit `.[]`); `select(.error_kind)` silently drops null rows — count
  with `// "none"` defaults or `wc -l`.
- **Census single-flight law:** ONE `gdd scan` at a time — concurrent
  scans share the forwarder and poison each other's unknown-rate
  measurements (proven: pilot+soak collision). Check
  `pgrep -f 'gdd[^ ]* scan'` first (tolerates dated binaries); waves run
  via `scripts/census-wave.sh` (detached; gate + cleanup + snapshot built
  in; watchdog `scripts/census-watchdog.sh` alongside; poller warns above
  the D3 bar via `GDD_WARN_PCT`).
- **Census binary provenance = the embedded VCS stamp** (`go version -m`),
  not `version` ("dev"); builds aren't byte-reproducible across dirs —
  record stamp + sha256 (`reports/census/unbound/PROVENANCE.md`).
- **crtsh-daily.sh wrong-root bug (fixed 2026-11-13):** the default data
  dir resolved OUTSIDE the repo into `/home/lars/projects/reports/census/
  zone-data` (a REAL sibling project — never delete that tree). Fixed
  path + plausibility guard (de ≥100k rows, `GDD_MIN_ROWS` override),
  branch-tested via the fixture; real verification = the next 04:37
  timer run landing in-repo. Lesson: test the LITERAL path string a
  script embeds, not a canonicalized equivalent.
- **Snapshot the journal with `sqlite3 <db> ".backup '<dest>'"`**
  (scripts/census-snapshot.sh) — WAL-consistent mid-run; a plain copy
  misses/stale-reads the -wal.
- `DomainObservedPayload` carries evidence only, never verdicts — verdicts
  are derived by `classify.Result` methods (split-brain rule for every
  event payload).
- **A hostile DNS record can poison journal decodability (2026-11-13,
  wave B):** invalid-UTF-8 TXT bytes made one event undecodable (breaking
  every journal fold) and its stdout render line refused — wedging the
  wave 4.5 h. Defense: encode-side UTF-8 coercion at the scan boundary
  (`classify.SanitizeEvidence`/`scan.sanitizeResult`, evidence copied not
  mutated) and before `event.New`, decode-side CBOR repair
  (`scanstore/cborsanitize.go`, fixture-pinned; journal stays
  append-only). Route NEW payload-carrying event types through the
  repair wrapper.
- Observation streams are per-domain (stream ID = domain name); run
  streams hold started/completed only. Replay keeps the LAST observation
  per domain per run and boots WITHOUT the projection host. Scan output
  preserves INPUT order, replay journal order — the e2e test compares
  parsed result SETS, never raw bytes.
- cqrs-lint: findings dispositioned inline with rationale at their site
  (2026-11-13, gate 0); package-level F005/F007 anchor to the
  alphabetically FIRST scanstore file; stale suppressions warn.
- Event payloads use `event.Instant` (not `time.Time` — CBOR epoch
  encoding drops timezones, C013) and every `event.New` stamps
  `event.WithSchemaVersion(1)` (D013). New events must follow both.
- Binary flag names: `--ranges-cache` (not `--cache-path`), `--event-store`
  (env `GDD_EVENT_STORE`). `--resume latest` resolves to the most recently
  STARTED run (an aborted soak/pilot counts) and silently skips its
  observed domains — wave A lost 2,731/100,000 to it; waves run fresh or
  `--resume <run-id>`, and `census-gate.sh --manifest` catches gaps (exit 4).
- Raising the go.mod `go` directive can break the INSTALLED buildflow
  binary (its analyzers run the Go they were built with — 2026-11-10
  toolchain.skew). `go.mod` floor is `go 1.27.2` and tidy ENFORCES it
  (go-cqrs-lite v4 tags); do NOT follow go-version-auto-configure's
  "major.minor only" advice. BuildFlow#56: go1.27.0-built binaries block
  the pre-commit hook — verify via direct gates and `--no-verify` for
  docs-only commits.
- docs-gates verdicts can differ between the pre-commit hook and a direct
  run (the hook runs `--build-mode pre-commit --staged-only` and stashes
  unstaged changes) — diagnose red-hook/green-direct with a direct run.
- Session start: run `gh run list --limit 3` before acting (the v0.1.0 tag
  sat on red CI for hours unnoticed). Release law: local green gates do
  NOT substitute for CI green on the exact pushed commit — tag only after
  the push-triggered CI run completes green; CI's race suite uses
  `-timeout 20m` (runners ~1.5–2× slower than local).
- gh's Go-resolved DNS dies intermittently on this host while SSH keeps
  working (2026-11-10): retry later or bypass with the documented
  `curl --resolve api.github.com:443:<IP>` + token pattern; SSH pushes are
  unaffected.
- BuildFlow's result cache replays FAILED step results verbatim — re-run
  with `BUILDFLOW_NO_RESULT_CACHE=1 -s <step>` before diagnosing the code.
  NEVER run two `-race ./...` suites concurrently on this host (phantom
  failures — one verification lane at a time).

## Git remotes

- `origin` → Forgejo `forgejo@evo-x2.home.lan:lars/google-domain-detector.git`
  (primary)
- `github` → `git@github.com:LarsArtmann/google-domain-detector.git` (mirror)

Push to both. The auto-commit daemon (pma) may commit mid-session — check
`git status` before committing; never assume.

Mixed-commit policy (daemon-swept bundles): junk-message bundles containing
unrelated concurrent changes → `git reset --soft <daemon-commit>^` +
per-file commits; message-only amend (`--no-verify`) ONLY when the tree is
byte-identical to the committed one. Never rewrite pushed history (mirror
included) without explicit approval.

## Downstream consumers (pointer)

- The Kith CRM lead-engine plan (crm/docs/planning/2026-11-10_14-59_*)
  consumes THIS repo's CLI outputs only: tier CSVs, rescans via `scan`
  (calibrated: forwarder 127.0.0.1:5353, workers 4 / timeout 15s — workers 8
  through the recursive forwarder fabricates a no-evidence class, plan §10
  erratum), and the reports/2026-11-09-tranco-de-ch-uk CSVs (gitignored).
- Sales artifacts: crm/docs/research/2026-11-10_google-scan-sales-targets.html
  + crm/docs/sales/. Census T16 landed: CRM delta logic retires in favor of
  `replay --since` deltas.
