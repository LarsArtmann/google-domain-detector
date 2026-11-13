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

## Library audit (2026-11-09)

> Point-in-time snapshot at the commit of `docs/status/`'s audit session —
> verdicts may be stale; re-verify against go.mod before acting on them.

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

- **Go's resolver conflates NXDOMAIN and NODATA into one `IsNotFound`
  error** — "no such host" for both an authoritative NXDOMAIN and an
  empty NOERROR, indistinguishable via the public API (Go 1.27 source:
  both construct `errNoSuchHost`). Any code that reads `IsNotFound` as
  "the domain is dead" stamps every live no-MX/no-evidence domain as
  NXDOMAIN — the 2026-11-13 census bug (~90% of wave A's 22,010 NX
  verdicts false; windowsupdate.com-class domains). Name existence MUST
  go through `resolve.LookupExistence` (rcode-level oracle); below an
  Exists verdict, sub-lookup IsNotFound is ordinary record absence, never
  an error (`scan.go` `isRecordAbsence`).
- **NEVER edit a script while a detached instance of it is mid-run** —
  bash reads scripts by byte offset, not line: an edit that shifts
  offsets under a running `census-wave.sh` makes the next post-child
  read land mid-content and aborts the wrapper with a bogus syntax
  error, orphaning the scan and its poller (2026-11-13: the nxfix wave
  lost its run-id/gate/cleanup/snapshot stages to exactly this; the
  scan survived and the stages were completed manually). Recovery
  recipe: check run1.stderr for the summary + run id, write `run-id`,
  re-run the gate with `--manifest`, run the cleanup round, snapshot,
  and log the incident in monitor.log. Related: `kill` in this shell is
  a silent no-op builtin — use `pkill -9 -f` (SIGTERM is also deferred
  by bash while it waits on a foreground child; kill the child's sleep
  or use -9).
- **DNS-over-TCP cannot be forced via Go's Dial callback** — the resolver
  frames messages by the network string IT passed in; overriding udp→tcp
  inside Dial sends unprefixed data. TCP transport to the resolver needs
  a hand-rolled wire client (which the existence oracle is) or a
  TCP-only listener the resolver reaches via its own TC-flag retry. The
  census TCP-upstream unbound variant (`scripts/unbound-tcp.conf`) keeps
  the client hop UDP and moves only unbound→upstream to TCP.
- **Failed-lookup error strings lie about the port**: DNSError.Server
  shows the resolv.conf address ("127.0.0.1:53") even when the scan's
  Dial rewrite sent the query to `--resolver 127.0.0.1:5353/5354` —
  `internal/resolve` discards Go's passed address (`_`) and dials its
  own. Never diagnose "the scan used the system resolver" from the error
  string; that caused a false protocol-contamination panic (2026-11-13).
- **metaengine dispatches ONE fold per (query, event type) — last registered
  wins** (`foldByEvent` overwrite, planner.go in metaengine v4.23.0). The
  observations query in `views.go` registers an insert-shaped AND an
  update-shaped fold for `scan.domain_observed`; only the update fold ever
  runs (MapUpdate invokes with nil prev on a missing key = upsert — pinned
  by `TestCommandsProjectReadModels` expecting `Observations == 1`). The
  insert fold is NOT dead weight: it declares the Map keyType that
  `QueryKeyType()` reads for dispatcher input-key extraction and feeds the
  planner's fold-kind ranking. Never "simplify" it away without re-running
  the read-model round-trip tests on memory AND sqlite.
- **G4 verdict (2026-11-10, plan v3):** the journal is trustworthy to at
  least 100k events — append 18.7s (5 345/s), replay 2.0s (51 035/s), db
  125 MiB, heap 15 MiB (`TestJournalOneHundredThousandEvents`, opt-in via
  `GDD_SOAK=1` — the soak exceeds CI's default 10-min test budget on
  2-core runners and is not a functional gate). Extrapolated
  to 2.5M domains: ~8 min append, ~3 GiB db, ~50s replay. Sink retries each
  observation 3× with backoff and reports the exact dropped count on Close;
  Ctrl-C mid-domain drains and still completes the run record (pinned by
  `TestScanCancelMidRunDrainsEventStore` with a deterministic cancel
  trigger — never wall-clock `AfterFunc`, which raced suite load: a cancel
  landing during event-store startup fails the run cleanly with exit 1,
  nothing journaled, and that is correct behavior). Whole-TLD runs are
  unblocked.
- **"Distressed resolver LIES NXDOMAIN" — SUPERSEDED 2026-11-13:** the
  2026-11-10/12 false-NXDOMAIN observations (T5's 14/14, the patch
  rounds' 6) had ONE root cause and it was NEVER the resolver: the
  scanner conflated Go's NXDOMAIN/NODATA `IsNotFound` (gotcha above);
  resolver path, cache, and load were falsified as factors (identical
  repro through a fresh-cache TCP-upstream instance). The DURABLE
  operational lessons from that incident stand: negative-cache entries
  PERSIST tens of minutes past their cause — never re-scan a poisoned
  bucket through the cache that watched the poisoning (the retracted
  "~90% genuinely dead" claim measured stale negatives), and positive
  verdicts were never affected (false negatives only). Bulk runs still
  MUST use `--resolver` against the census forwarder, never the system
  resolver.
- **Host-level outbound UDP:53 outages happen (2026-11-12 23:24–23:45):**
  every fresh UDP recursion died (`dig @9.9.9.9` = no servers reachable)
  while TCP:53 and ICMP kept working; a wave launched into the window
  burned ~3 450 garbage observations. `census-wave.sh` therefore runs a
  REAL-UPSTREAM PRE-FLIGHT (exit 5 on failure; forwarder liveness alone
  proves nothing — it answers from cache while upstream is dead) and
  waves on the TCP-upstream forwarder set `GDD_PREFLIGHT_TRANSPORT=tcp`
  so a UDP-only outage cannot block a launch immune to it.
- **This host runs a live resolver on `[::1]:53`** — discovered when the
  IPv6 resolver e2e test answered from it; `[::1]` loopback is NOT a
  dead network on this machine, and "port 53 in use" style surprises can
  come from v6 loopback, not just 127.0.0.1.
- cmdguard's flag-binding layer returns `fmt.Errorf`-wrapped SENTINELS
  (`v5.ErrInvalidEnum`, `v5.ErrValueTooLarge`, …), not `*v5.EnumError`/
  `*ExitError` structs — `errors.As` for the struct types finds nothing.
  `internal/cli.Run` maps that sentinel family to `usageError` so bad flag
  values exit 2 (see `isFlagValueError`); new flag-level usage classes must
  join that list.
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
- **jq on this host is a SHIM**, not real jq: it breaks on relative paths
  in some invocations (prefer absolute) and does NOT auto-iterate arrays
  (always explicit `.[]`). Also remember plain jq semantics under it:
  `select(.error_kind)` silently drops rows where the field is null
  (clean scan results!) — count with `// "none"` defaulting or `wc -l`
  when a number matters.
- **Census single-flight law (2026-11-11):** ONE `gdd scan` at a time —
  concurrent scans share the forwarder on 127.0.0.1:5353 and poison each
  other's unknown-rate measurements (proven: pilot + soak collision,
  docs/census.md). Check `pgrep -f 'gdd[^ ]* scan'` before launching
  (the pattern must tolerate dated binaries like `gdd-20261113` — a
  literal `gdd scan` matches nothing for them and silently breaks the
  guard); waves
  run via `scripts/census-wave.sh` (detached, gate + cleanup built in,
  auto-snapshots the journal into `reports/census/snapshots/` on
  completion; poller warns above the D3 bar via `GDD_WARN_PCT`).
- **Editing a RUNNING bash script corrupts it** — bash reads scripts by
  byte offset on the SAME inode; in-place rewrites make the running
  instance execute garbage after its current command. Replace via a new
  inode instead (`install -m 755 new old` — unlink+create): the running
  instance keeps the old inode and finishes on the old code. Used for
  census-wave.sh mid-wave-A (2026-11-11); verified by inode + pid checks.
  SHARPENED (2026-11-13, mid-wave-B near-miss): the edit/write tools
  rewrite the SAME inode — pointing them at a running bash script shifts
  its pending byte offset instantly (caught via /proc/<pid>/fd/255 sharing
  the path's inode). Recovery that worked: revert the edit to byte-identical
  original (verify with `cmp` vs `git show HEAD:`), then build the real
  change in /tmp and install it over the path (new inode).
- **Census binary provenance = the embedded VCS stamp**, not `version`
  (which prints only "dev"): `go version -m <binary>` exposes
  `vcs.revision` + `vcs.modified` + toolchain — authoritative even when
  the build wasn't stamped. Byte-reproducing a build from another
  directory is impossible (build-dir paths embedded), so don't try;
  record the stamp + sha256 instead (`reports/census/unbound/PROVENANCE.md`).
- **crtsh-daily.sh wrong-root bug (fixed 2026-11-13):** the default
  `data_dir` was `$here/../..`-based, which resolved OUTSIDE the repo into
  `/home/lars/projects/reports/census/zone-data` (a REAL sibling project —
  never delete that tree; only its stray `census/` dir was crtsh fallout).
  The 04:48 timer run landed there; artifacts (ch 1,685 / de 2,060 rows in
  ~18 s) were truncated — relocated into the repo zone-data dir, monitor.log
  merged. Wrapper now carries a plausibility guard (de ≥100k rows, other
  TLDs ≥1k, `GDD_MIN_ROWS` override) branch-tested via the crtsh fixture.
  Real verification = the next 04:37 timer run landing in-repo. Lesson:
  test the LITERAL path string a script embeds, not a canonicalized
  equivalent — isolated probes masked the bug for three debug rounds.
- **Snapshotting the journal: use `sqlite3 <db> ".backup '<dest>'"`**
  (scripts/census-snapshot.sh) — WAL-consistent and safe while a scan is
  mid-run; a plain file-copy of census.db misses/stale-reads the -wal.
  Restore-verified: tar → integrity_check → row counts.
- `DomainObservedPayload` carries evidence only, never verdicts — verdicts
  are derived by `classify.Result` methods; duplicating them in events would
  be a split brain. Same rule for any new event payload.
- **A hostile DNS record can poison journal decodability (2026-11-13,
  wave B):** one .com's google-site-verification TXT carried raw
  invalid-UTF-8 bytes; CBOR text strings must be valid UTF-8, so the
  event decoded fine at write time and broke EVERY journal fold that
  reached it (`corruption:event.decode_payload_failed`). Defense is
  two-sided in `internal/scanstore/cborsanitize.go`: encode-side
  `sanitizeObservedPayload` coerces all DNS-derived strings to valid
  UTF-8 before `event.New` (evidence copied, never mutated — the slice
  aliases the scan result other sinks read); decode-side
  `decodeObserved` repairs a failed payload once by rewriting invalid
  CBOR text strings in place (`sanitizeCBORText`, fixture-pinned to the
  real poison event) — the journal stays append-only. If a NEW
  payload-carrying event type appears, route its decode through the
  repair wrapper.
- Observation streams are per-domain (stable stream ID = domain name), so
  whole-TLD scale never bloats one stream; run streams hold only
  started/completed. Replay keeps the LAST observation per domain per run.
- `replay` boots the system WITHOUT the projection host (`scanstore.Boot`)
  — journal replay must not pay for folding every read model first.
- Ordering contract: scan output preserves INPUT order; replay preserves
  journal (completion) order. Same facts, different orderings — the e2e
  test compares parsed result SETS, never raw bytes with >1 domain.
- cqrs-lint deliberate non-fixes: E003 (scanstore mixes command/event/fold
  in one package — warning) — the package owns ONE concern (scan event
  sourcing) in a 4-file split; a domain/infrastructure package split is
  over-engineering for a single-consumer internal package. B005
  (StrictApply — info): folds intentionally ignore unknown event types;
  new event types must not crash old binaries replaying old journals.
- Event payloads use `event.Instant` (not `time.Time` — CBOR epoch encoding
  drops timezones, C013) and every `event.New` stamps
  `event.WithSchemaVersion(1)` so the first schema evolution can upcast
  (D013). New events must follow both.
- Binary flag name is `--ranges-cache` (not `--cache-path`).
- `--resume latest` resolves to the most recently STARTED journal run
  (an aborted soak/pilot counts) and silently skips its observed domains
  — wave A lost 2,731/100,000 to it. Waves run fresh or
  `--resume <run-id>`; `census-gate.sh --manifest` catches gaps (exit 4).
- Binary flag for the SQLite journal is `--event-store` (env `GDD_EVENT_STORE`).
- Raising the go.mod `go` directive (dependency floor) can break the INSTALLED
  buildflow binary: its embedded analyzers run the Go they were built with
  (observed 2026-11-10: go1.27.0 binary vs `go 1.27.2` floor → samber-linter
  `toolchain.skew` + test-compile `stale_toolchain` both fail). Fix is
  rebuilding/refreshing the buildflow binary, not repo changes.
- docs-gates verdicts can legitimately differ between the pre-commit hook and
  a direct run: the hook runs `--build-mode pre-commit --staged-only` and
  stashes unstaged changes, so it evaluates a different file set (plus result
  cache keys are per matched-file set). Diagnose red-hook/green-direct with a
  direct `docs-gates freshness` run before suspecting the gate itself.

- `go.mod` floor is `go 1.27.2` and tidy ENFORCES it (go-cqrs-lite v4 tags
  require >= 1.27.2). Known tool bug (BuildFlow#56): buildflow binaries
  built with go1.27.0 run embedded analyzers (test-compile, samber-linter,
  go-generate) that cannot load a module with this floor - the pre-commit
  hook blocks until BuildFlow is rebuilt with a newer toolchain. Until then:
  verify via direct gate runs (docs-gates, go build/test, nix build) and use
  `--no-verify` for docs-only commits. Do NOT follow
  go-version-auto-configure's "major.minor only" advice here - `go mod
  tidy` reverts it; it cannot stick while the cqrs pins are current.
- Session start: run `gh run list --limit 3` before acting — the v0.1.0
  tag sat on a commit whose CI was red (soak timeout) for hours unnoticed.
- Release law (learned 2026-11-10, §d1 of the 13-27 report): local green
  gates do NOT substitute for CI green on the exact pushed commit — tag
  only AFTER the push-triggered CI run completes green. CI's race suite
  runs with `-timeout 20m` for the same reason (runners are ~1.5–2×
  slower than local; the 10m default panicked mid-suite).
- gh's Go-resolved DNS dies intermittently on this host while SSH and
  glibc tools keep working (2026-11-10 outage). If `gh` fails to reach
  api.github.com: retry later, or bypass with the documented
  `curl --resolve api.github.com:443:<IP>` + token pattern; pushes via
  SSH (`github` remote) are unaffected.
- BuildFlow's result cache replays FAILED step results verbatim
  (2026-11-13): a `test-race` step that "fails" in seconds with
  stale-looking output is a cached failure, not a fresh one — re-run
  with `BUILDFLOW_NO_RESULT_CACHE=1 -s <step>` before diagnosing the
  code. The root failure that seeded the cache was resource contention
  from running two `-race ./...` suites at once; one verification lane
  at a time on this host.
- NEVER run your own `go test -race ./...` concurrently with a buildflow
  gate (or another suite) on this host — the interference produced a
  phantom cli-package failure that cost a full diagnosis round-trip
  (2026-11-13, see the 09-57 and 10-23 status reports §d).

## Git remotes

- `origin` → Forgejo `forgejo@evo-x2.home.lan:lars/google-domain-detector.git`
  (primary)
- `github` → `git@github.com:LarsArtmann/google-domain-detector.git` (mirror)

Push to both. The auto-commit daemon (pma) may commit mid-session — check
`git status` before committing; never assume.

Mixed-commit policy (daemon-swept bundles): when the daemon has already
committed your content under a junk message AND bundled unrelated concurrent
changes, prefer `git reset --soft <daemon-commit>^` + per-file commits over a
message-only amend of the bundle — one-commit-one-authorship beats a flattering
message on murky content. Message-only amend (`--no-verify`) is acceptable ONLY
when the tree is byte-identical to the already-committed tree. Never rewrite
pushed history (mirror included) without explicit approval.

## Downstream consumers (pointer)

- The Kith CRM lead-engine plan (`crm/docs/planning/2026-11-10_14-59_SUPERB-lead-engine-first-contact-pareto-plan.md`)
  consumes THIS repo's CLI outputs only (guard 3 of that plan): tier CSVs from
  `gdd-final.json`/`gcp-tiers.json`, rescans via the `scan` subcommand (the
  unbound forwarder 127.0.0.1:5353 + workers 4 / timeout 15s is the calibrated
  setting — workers 8 through the recursive forwarder fabricates a large
  "no-evidence" class, see that plan's §10 erratum), and the wave-1/2 target
  CSVs under `reports/2026-11-09-tranco-de-ch-uk/` (gitignored data).
- Sales-side artifacts: `crm/docs/research/2026-11-10_google-scan-sales-targets.html`
  (the 150 A+B report), `crm/docs/sales/` (openers, playbook, wave handoffs).
- Delta rescans (CRM M11) stay CLI-consumed until census T16 lands; when it
  does, the CRM script's diff logic retires in favor of `replay`-based deltas.
