# Features

Honest inventory of what google-domain-detector does, by status.
Last refreshed: 2026-11-10 (pre-v0.1.0).

## DONE

### Detection

- Batch DNS probe of apex + `www` per domain: MX, TXT/SPF,
  `google-site-verification`, CNAME, A/AAAA, NS, and a conditional
  `google._domainkey` DKIM probe (only when MX+SPF are silent) behind a
  bounded worker pool with per-domain timeouts and NXDOMAIN short-circuit.
- Workspace verdicts: MX to google.com/googlemail.com → **certain**;
  SPF include / DKIM → **likely** (free Gmail "send as" never upgraded).
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
- Formats: table, json (full-evidence array), jsonl (streaming, completion
  order), csv, tsv, markdown, yaml, toml — enum drift-pinned to the
  renderer registries by test.
- Filters (`--only workspace|gcp|google|all`), colors
  (`--color auto|always|never`, NO_COLOR), totals footer on table/markdown,
  live per-domain progress on TTYs (pipes/jsonl stay byte-identical).
- Scriptable exit codes: usage errors exit 2 (incl. bad flag values);
  range-feed failures carry 75 (fetch) / 65 (corrupt).

### Journal (go-cqrs-lite)

- `scan --event-store`: append-only SQLite journal, one stream per domain
  (re-scans keep history) + one per run; sink retries 3× and reports
  dropped counts loudly.
- `replay`: rebuild any run's report from the journal alone (zero DNS).
- `show <domain>` / `runs`: read-model queries (observation history, run
  ledger with resumed-from links).
- `--resume ID|latest`: the journal IS the checkpoint.
- Ctrl-C drains: completed observations recorded, run record completed.
- Trust benchmark: 100k events — append ~5.3k/s, replay ~51k/s, 125 MiB.

### Ops

- `doctor` preflights: resolver answers (GDD_RESOLVER-aware), range feed
  loads (names the 75/65 family).
- GCP range feed: 24h disk cache (crash-safe atomic writes, 0600),
  stale-cache fallback, identifying User-Agent, `--ranges-url`/
  `--ranges-cache` overrides.
- `--resolver host[:port]` / `GDD_RESOLVER` for bulk runs.
- `describe` battery: machine-readable CLI contract; `--wave` stamps
  census waves into the journal.
- Offline test suite (fake resolvers, httptest feeds); nix flake build;
  BuildFlow-covered.

## PARTIALLY DONE

- Whole-TLD census: pilot artifacts and plans exist
  (docs/planning/2026-11-10_06-18_*); unbound spike verified resolving;
  no wave has run yet.
- .ch zone access: SWITCH public TSIG path verified and documented
  (docs/zone-data/switch-ch.md); the pull itself not yet done.

## PLANNED

- Homebrew tap (post-tag), asciinema demo.
- Census waves A/B/C on the Tranco top-1M with pilot gates, cleanup loops,
  census report, verdict-flip delta query.
- NXDOMAIN pre-sweep / A-existence filter for whole-TLD scale.
- CSV enrichment mode (append verdict columns to the input CSV).
- crt.sh fallback extractor for .de coverage (DENIC refuses zone access).

## WORTH CONSIDERING (parked)

- CAA / other record classes — only if a real fingerprint emerges.
- Journal compaction for multi-million-event stores.
- Lead scoring (workspace-certain × GCP × Tranco rank).
