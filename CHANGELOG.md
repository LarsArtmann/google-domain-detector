# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [Unreleased]

### Added

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
