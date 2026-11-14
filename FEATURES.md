# Features

Honest inventory of what google-domain-detector does, by status.
Last refreshed: 2026-11-14 (post-wave-A, during wave B).

## FULLY_FUNCTIONAL

### Detection

- Batch DNS probe of apex + `www` per domain: MX, TXT/SPF,
  `google-site-verification`, CNAME, A/AAAA, NS, and a conditional
  `google._domainkey` DKIM probe (only when MX+SPF are silent) behind a
  bounded worker pool with per-domain timeouts and NXDOMAIN short-circuit.
- Name-existence oracle (`resolve.LookupExistence`): rcode-level wire
  query decides domain existence, immune to Go's NXDOMAIN/NODATA
  `IsNotFound` conflation (~90% of one census wave's NX verdicts were
  false before it); below an Exists verdict, record absence is ordinary
  absence, never an error.
- Workspace verdicts: MX to google.com/googlemail.com → **certain**;
  SPF include / DKIM → **likely** (free Gmail "send as" never upgraded).
- Microsoft 365 fingerprints (evidence-only): MX under
  mail.protection.outlook.com and SPF include of spf.protection.outlook.com
  are recorded as `microsoft`-platform evidence so one scan paints the full
  email-stack picture — Microsoft evidence never upgrades a Google verdict.
- GCP verdicts: CNAMEs to run.app / appspot.com / web.app / firebaseapp.com
  / cloudfunctions.net / googlehosted.com / storage.googleapis.com, NS under
  googledomains.com, and A/AAAA inside the published `cloud.json` GCP
  ranges (netip, v4-in-6 unmapped) — all **certain**.
- Error taxonomy end to end: nxdomain / timeout / servfail / canceled
  classes in the summary, JSON DTO (`error_kind`), and the journal.
- Transient-failure retry (timeout/SERVFAIL once); NXDOMAIN never retried.

### Input & output

- Inputs: arguments, `--input FILE` (plain lists and CSV with a recognized
  domain column), stdin; dedup, first-seen order; bad rows skipped+counted.
- Chunked streaming input (4096/chunk): million-row lists never load whole.
- `enrich`: join the event-sourced journal back onto your input CSV —
  verdict columns appended, every original column preserved (no DNS in
  the join).
- Formats: table, json (full-evidence array), jsonl (streaming, completion
  order), csv, tsv, markdown, yaml, toml — enum drift-pinned to the
  renderer registries by test.
- `runs --format`: data formats carry FULL run IDs + raw run fields;
  table/markdown keep short IDs; data formats stay footer-free.
- Filters (`--only workspace|gcp|google|all`), colors
  (`--color auto|always|never`, NO_COLOR), totals footer on table/markdown,
  live per-domain progress on TTYs (pipes/jsonl stay byte-identical),
  `--max-runtime` whole-run budget (partial results still render, journal,
  and count; the summary names the never-scanned gap).
- Scriptable exit codes: usage errors exit 2 (incl. bad flag values);
  range-feed failures carry 75 (fetch) / 65 (corrupt).

### Journal (go-cqrs-lite)

- `scan --event-store`: append-only SQLite journal, one stream per domain
  (re-scans keep history) + one per run; sink retries 3× and reports
  dropped counts loudly; hostile record content is sanitized at the
  encode boundary and repaired at decode (invalid-UTF-8 TXT cannot poison
  the journal or the render path).
- `replay`: rebuild any run's report from the journal alone (zero DNS).
- `replay --since RUN_ID`: verdict-flip delta between runs (adoption /
  departure signal; errored lookups count as unknown, never departures).
- `replay --audit-excluded`: supersession audit — every domain whose
  last observation comes from an excluded run (run IDs or wave names),
  the mechanical poison check.
- `show <domain>` / `runs`: read-model queries (observation history, run
  ledger with resumed-from links); read-model commands wait for
  projection workers to settle before querying (no stale/empty ledgers
  after an unclean close).
- `--resume ID|latest`: the journal IS the checkpoint.
- `--wave` stamps census waves into the journal; ruleset ID stamped on
  every run + observation (detection provenance).
- Ctrl-C drains: completed observations recorded, run record completed.
- Trust benchmark: 100k events — append ~5.3k/s, replay ~51k/s, 125 MiB.

### Ops

- `doctor` preflights: resolver answers (GDD_RESOLVER-aware), range feed
  loads (names the 75/65 family).
- GCP range feed: 24h disk cache (crash-safe atomic writes, 0600),
  stale-cache fallback, identifying User-Agent, `--ranges-url`/
  `--ranges-cache` overrides.
- `--resolver host[:port]` / `GDD_RESOLVER` for bulk runs (IPv6 addresses
  test-covered).
- `describe` battery: machine-readable CLI contract.
- Census machinery: `scripts/census-wave.sh` (detached waves with gate +
  cleanup + snapshots), external watchdog + heartbeat (`census-watchdog.sh`),
  manifest-coverage gate (`census-gate.sh --manifest`), unbound resolver
  lifecycle (`scripts/unbound.sh`, forwarder + TCP-upstream variants).
- Zone inputs: `.ch` zone pulled via public SWITCH TSIG keys (2,660,632
  apexes, sha256-pinned); daily crt.sh fallback for `.de` (systemd timer,
  plausibility-guarded).
- Distribution: `go install @latest`, Homebrew tap
  (`LarsArtmann/tap/google-domain-detector`), nix flake hermetic build.
- Offline test suite (fake resolvers, httptest feeds); BuildFlow-covered.

## PARTIALLY_FUNCTIONAL

- Whole-TLD census (Tranco top-1M): wave A (ranks 1–100k) complete —
  pass-1 health PASS (unknown 1.53%), false-NXDOMAIN correction wave
  (`tranco1m-a-nxfix`) done, supersession audit = 25 poisoned domains
  queued for the correction pass. Wave B (100k–500k) running since
  2026-11-13. Wave C, poisonfix-25, and the census report's final
  distribution section chain behind B's gate (single-flight law).
  Census report skeleton: reports/census/REPORT.md.
- `.ch` scanning: zone input ready (2.66M apexes); the scan itself waits
  for the Tranco waves to free the single-flight.
- `.uk` zone access: Nominet application draft ready for owner review
  (docs/zone-data/nominet-uk.md); sending = owner action, scanning
  follows Files.com credentials.

## PLANNED

- asciinema demo / GIF for the README.
- Wave C (500k–1M) + poisonfix-25 correction + census closeout report.

## WORTH CONSIDERING (parked)

- CAA / other record classes — only if a real fingerprint emerges.
- Journal compaction for multi-million-event stores.
- Lead scoring (workspace-certain × GCP × Tranco rank).
