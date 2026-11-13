# Census binary provenance

Recorded 2026-11-11 retroactively (the build was not stamped at build
time; status report 2026-11-11_15-27 §e2). The binary's embedded VCS
stamp (`go version -m`) is the authoritative record.

- binary: `reports/census/unbound/gdd`
- sha256: `4b2cb1a87a1d73540af0123d3be7af148a6bb227c699bf40364c2b6554f77a64`
- built: 2026-11-10 20:15 CET (mtime); `vcs.time` 2026-11-10T18:12:02Z
  (= commit time of e1f5160)
- `vcs.revision`: `e1f5160caf8e951abce5a697178d6a6073237e50` (e1f5160,
  "Status report: E6 A/B executed, release verifications closed, E24
  lint sweep")
- `vcs.modified`: true — uncommitted changes existed at build time;
  most plausibly the 8-file set the auto-commit daemon captured as
  01fe9fa at 20:17 (two minutes after the build mtime). Not
  byte-verifiable: the original build ran in the repo root and Go
  embeds build-dir paths, so worktree rebuilds can never hash-match
  (attempted from both 01fe9fa clean and e1f5160+dirty; stamps and
  paths differ).
- toolchain: go1.27.2, GOEXPERIMENT=jsonv2, CGO_ENABLED=1, linux/amd64
- `version` output: `google-domain-detector dev` (no commit embedded in
  the version string; the VCS stamp above is the provenance)

Decision (2026-11-11): waves B and C reuse THIS binary, same sha256 —
one instrument for the whole census. Do not rebuild mid-census; a
detection-rule change would require a ruleset bump plus a full new
census, not a silent binary swap.

Verification recipe for any future session:

    sha256sum reports/census/unbound/gdd
    go version -m reports/census/unbound/gdd | grep vcs

## gdd-20261112 (wave-A patch + waves B/C)

- binary: `reports/census/unbound/gdd-20261112`
- sha256: `9124f832625f0a5c8d5f079ede34dee66dbcaa539577d971afc13531240421c8`
- built: 2026-11-12 23:2x CET from the repo root, clean tree
- `vcs.revision`: `308c32d6bc6d69b202707eecbd7e0c76b78d77b5` (308c32d,
  "Add journal-backed CSV enrichment (enrich command)")
- `vcs.modified`: false — first fully-clean census build
- behavior deltas vs wave-A binary (e1f5160, 2026-11-10): interrupted-run
  summary counts scanned-only + "interrupted:" line; resume prints skip
  accounting; scan --max-runtime budget; enrich command; detection rules
  UNCHANGED (classify.Ruleset stays v1-2026-11-10 — no ruleset bump needed).

## gdd-20261113 (false-NXDOMAIN fix; tranco1m-a-nxfix correction wave + waves B/C)

- binary: `reports/census/unbound/gdd-20261113`
- sha256: `13dd192d05b08ca70cff80c20891495860877c509f034f55821a8c5d90950e0a`
- built: 2026-11-13 02:5x CET, clean tree (`vcs.modified=false`)
- `vcs.revision`: `7b45a1b` — "Fix false NXDOMAIN verdicts: name
  existence now read at rcode level" (existence oracle + NODATA-is-
  absence semantics; root cause and evidence in docs/census.md
  "false-NXDOMAIN root cause")
- toolchain: go1.27.3, GOEXPERIMENT=jsonv2
- Detection rules UNCHANGED — ruleset stays v1-2026-11-10; the fix
  changes error semantics only (NODATA is no longer NXDOMAIN). Runs
  from this binary are distinguished by wave id (`tranco1m-a-nxfix`),
  not by a ruleset bump.
- Supersedes gdd-20261112 for all future waves (B/C + correction waves).

## gdd-20261113b (2026-11-13 16:53)

- vcs.revision 30441708fcfda21a8c1f837671ec8f98ad48618e (clean tree),
  sha256 798f9c7ad2653fc9…
- Content: UTF-8 sanitizer at the scan boundary (classify.SanitizeEvidence +
  scan.sanitizeResult) + CBOR decode-side repair (scanstore.cborsanitize).
- Supersedes gdd-20261113 for ALL future waves: gdd-20261113's render path
  died on the bide-et-musique.com hostile TXT (jsontext: invalid UTF-8),
  wedging tranco1m-b pass 1 for 4h21m (stdout frozen 11:52:49, journal
  writes stalled ~16:13, SIGINT drained cleanly 16:46 — run
  01M79XW7SF918E61NYVM000001, 96,215 domains journaled).
