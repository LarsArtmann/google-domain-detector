# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [Unreleased]

### Added

- `replay --audit-excluded RUN_ID[,RUN_ID...]`: the supersession audit —
  lists every domain whose last journal observation comes from an
  excluded run, the mechanical proof that correction passes covered the
  poisoned supersessions (findings are data: exit 0 either way, counts
  on stderr; unknown run IDs fail the audit instead of reporting clean).
  Exclusions also accept wave names (bare or `wave:NAME`), expanded from
  the journal's run stamps — the audit stays independent of projection
  read models.
- `replay --since RUN_ID`: delta mode reporting exactly the domains whose
  Workspace/GCP verdict flipped since an earlier run (adopted/left counts
  on stderr, all output formats incl. streaming jsonl). Errored lookups
  count as unknown — never as a departure; flips across different
  detection rulesets carry an explicit warning.
- Census tooling: `scripts/unbound.sh` lifecycle (user-scoped caching
  forwarder on 127.0.0.1:5353), pilot gate `scripts/census-gate.sh`
  (unknown-rate bar from a run summary or jsonl), Tranco manifest
  freezer `scripts/tranco-manifests.sh`, and the census runbook
  `docs/census.md` (resolver design, wave ledger, error semantics).
- Nightly `Soak` workflow runs the opt-in 100k-event journal soak
  (GDD_SOAK=1) on a schedule; CI's race step now carries `-timeout 20m`
  (runners are slower than local and the 10m default panicked mid-suite).

### Fixed

- `replay` run listings were silently truncated at 100 runs: metaengine
  `Scan` caps results at 100 unless told otherwise, and `ListRuns` (the
  `--resume`/`--run` selection surface) passed no limit — a journal with
  more runs would hide its oldest runs from every listing. Now explicitly
  unbounded (`WithLimit(0)`), matching the documented contract.
- A failed run start inside the event store closed the store but
  swallowed its close error (pending events could be lost silently);
  it is now reported to stderr like every other close failure.
- cqrs-lint findings driven to zero: the remaining 12 (A032/B004/B005/
  C013/C015/E003/F005/F007) are dispositioned as justified suppressions
  with rationale at each firing site — none silent.

- False NXDOMAIN verdicts eliminated: Go's resolver conflates NXDOMAIN
  and NODATA into a single `IsNotFound` error, and the scanner stamped
  both as domain death (~90% of wave A's 22,010 NX verdicts were false;
  windowsupdate.com-class domains alive at Quad9). Name existence now
  goes through `resolve.LookupExistence`, a wire-level rcode oracle;
  below an Exists verdict, record absence is ordinary absence, never an
  error. Detection rules are unchanged (ruleset v1-2026-11-10).
- `replay --run <unknown-id>` silently returned an empty report; it now
  fails naming the run and store (new `scanstore.ErrUnknownRun`), as does
  `--since` with an unknown baseline.

## [0.1.0] - 2026-11-10

First tagged release. Validation: 3 645-domain `.ch` slice scanned clean
(G3 PASS — journal round-trip byte-identical, independent dig
cross-checks, error tail fully explained by taxonomy; see
docs/status/2026-11-10_07-46 — its NXDOMAIN-tail conclusion is corrected
by reports/2026-11-10-e1-validation/README.md: shared-cache negatives
after resolver distress are unreliable, 11/12 sampled "dead" domains were
alive). Rollback plan if a fatal defect is found:
add a `retract v0.1.0` directive to go.mod and tag v0.1.1 — the module
proxy is immutable, re-tagging is never the answer.

### Added

- Explained error tails: every result error is classified (nxdomain, timeout,
  servfail, canceled) end to end — scan summary, replay summary, JSON DTO
  (`error_kind`), and the event journal all carry it. Transient lookup
  failures (timeouts, SERVFAIL) are retried once; authoritative NXDOMAIN
  answers never are. `scan` now prints a one-line run summary to stderr
  ("N domains: workspace certain X / likely Y, gcp Z, errors E (nxdomain a,
  timeout b, ...)") so bulk-run health is readable without post-processing.
- Scriptable exit codes: bad invocations (no domains given, replay without
  an event store, a flag value outside its allowed set — bad `--color`,
  `--workers 513`, malformed `--timeout`) exit 2, scan failures exit 1.
  `--workers` is validated at
  parse time (1–512); env vars resolve under a single `GDD_` prefix
  (`GDD_RESOLVER`, `GDD_EVENT_STORE`, `GDD_RANGES_CACHE`); panics surface as
  printed errors instead of crashes (RecoveryMiddleware); version output can
  carry the build commit (`-X main.commit=...`), and `scan --help` gained a
  usage example block.
- Crash-safe GCP range-feed cache writes via go-atomic-write `WriteWithPerm`
  (0600): a killed run can no longer leave a torn `cloud.json` behind, and
  identical feed rewrites still refresh the cache mtime the 24 h TTL reads.
  The feed fetch now sends `User-Agent: google-domain-detector/<version>`.
- `--format yaml`, `--format toml`, and `--format jsonl` on both `scan` and
  `replay`: the renderers were already compiled in (go-output serialization
  blank imports) but unreachable through the flag enum. `jsonl` emits the
  full-evidence JSON DTO one object per line — the same shape as `json`
  without the array wrapper, ready for line-by-line consumption.
- Event-sourced scans via go-cqrs-lite (`system` + `metaengine`, SQLite
  engine): `scan --event-store FILE.db` journals every run — one stream per
  domain (re-scans keep history), one per run — and the new `replay` command
  rebuilds any run's report from the journal alone, zero DNS. Events carry
  evidence only; verdicts are always recomputed from it.
- `--resolver`/`GDD_RESOLVER`: query a given DNS server (host or host:port)
  directly instead of the system resolver — for bulk scans that must not ride
  the local forwarding resolver.
- Library audit docs: `docs/review/libs/` — one utilization report per
  LarsArtmann library (22 audited) with adopt/skip verdicts and scored
  opportunities.
- Streaming output: `--format jsonl` on `scan` now writes each result the
  moment its domain completes (completion order) instead of after the whole
  run — a canceled run keeps every line already written; domains never
  started emit nothing. A 50k-domain smoke run holds ~40 MiB RSS flat.
- Streaming input: input parsing is chunked (4096 domains at a time, plain
  lists and CSV alike), so million-row inputs never sit fully in memory and
  jsonl runs are bounded end to end. `replay --format jsonl` streams
  straight out of the journal walk — a whole-TLD store never materializes.
- Journal-based resume: `scan --resume <runID>|latest --event-store FILE`
  skips every domain that run already observed (interrupted runs included)
  and starts a new run linked back via `resumed_from` in `scan.started`.
  The journal IS the checkpoint — no half-written output files.
- Trustworthy recording: the event-store sink retries each observation
  (3 attempts, backoff) and, if the store stays broken, fails loudly naming
  the exact count of dropped observations — never a silent journal gap.
  Ctrl-C mid-scan drains and still completes the run record. The journal is
  benchmark-trusted to 100k events (append ~5.3k/s, replay ~51k/s, 125 MiB
  db) — extrapolated ~8 min append / ~3 GiB for a 2.5M-domain zone.
- `show <domain>` and `runs` commands: query the event store's read models —
  full observation history per domain and the run ledger (started, counts,
  resumed-from links). The CQRS loop is closed: journal in, read models out.
  The runs ledger table carries a title and an `N run(s)` totals footer.
- `--color auto|always|never` on `scan` and `replay` for table and markdown
  output (`auto` respects `NO_COLOR`; `--no-color` is the never shorthand).
  Table and markdown gain a totals footer (`N domains / M workspace / K gcp`);
  csv and tsv stay pure data so downstream tools never see a totals row.
- `doctor` preflights the two external dependencies a scan needs — a
  `dns-resolver` check (a real lookup inside a 5s budget, honoring
  `GDD_RESOLVER`) and a `ranges-feed` check (the feed loads within 10s from
  network or cache, naming the exit-75/65 failure family) — so a broken
  environment is found before a bulk run wastes minutes on it.
- Live progress on interactive terminals: `scan` renders an in-place
  activity tree (one line per domain, ✓ done / ✗ failed, running count at
  a glance) via go-output's `nom` inline renderer while the scan runs —
  then clears it and prints the report. Pipes and `--format jsonl` keep
  byte-identical output to before; progress is TTY-only by construction.

- `scan` command: batch DNS probe of apex + www per domain (MX, TXT/SPF,
  google-site-verification, CNAME, A/AAAA, NS, conditional DKIM) behind a
  bounded worker pool with per-domain timeout and NXDOMAIN short-circuit.
- Workspace verdicts (MX → certain, SPF/DKIM → likely) and GCP verdicts
  (run.app, appspot.com, web.app, firebaseapp.com, cloudfunctions.net,
  googlehosted.com, storage.googleapis.com CNAMEs; googledomains.com NS;
  cloud.json IP ranges).
- Input: arguments, `--input FILE` / `-` / piped stdin; plain lists and CSV
  with a recognized domain column; unparseable rows skipped and counted.
- Output: table (default), compact full-evidence JSON, csv, tsv, markdown;
  `--only workspace|gcp|google|all` filter.
- GCP range feed fetch with 24 h disk cache and stale-cache fallback.
- cmdguard v5 command surface (`scan`, plus version/doctor/describe
  batteries); go-output rendering; nix flake via go-nix-helpers; BuildFlow
  coverage; offline test suite (fake resolvers, httptest feeds).
