# TODO List

Short- and mid-term improvement tasks for google-domain-detector. Status
markers: `[ ]` open, `[~]` in progress, `[x]` done. Done rows are deleted
on sight — completed work lives in CHANGELOG.md and the status reports,
never here.

## Census: Tranco top-1M (plan: docs/planning/2026-11-10_06-18_tranco-1m-census-event-sourced.md; execution canon: docs/planning/2026-11-14_00-42_SUPERB-census-closeout-pareto-plan.md)

Single-flight law: exactly ONE `gdd scan` at a time
(`pgrep -f 'gdd[^ ]* scan'` before every launch). Chain order:
B → B gate → (T14 if FAIL) → wave C → poisonfix-25 → closure audit →
T15 final → closing report.

- [~] T12–T13 Waves B (100–500k), C (500k–1M) with pilot gates — B
  attempt 2 RUNNING since 2026-11-13 16:53 (gdd-20261113b, explicit
  resume of `01M79XW7SF918E61NYVM000001`, expect-skip 96,215,
  TCP-upstream forwarder 127.0.0.1:5354, w4/10s; watchdog attached,
  0 alarms through 2026-11-14 01:49; wrapper auto-gates with
  `--expect-skip 96215` on completion). C launches after B's gate PASS.
- [ ] T11-poisonfix Correction pass for the poisoned supersessions —
      READY (audit-derived **25 domains**: c4 10 + c5 15; pilot/soak3 0):
      input = `reports/census/waves/tranco1m-a-poisonfix.txt`, fresh
      binary, after wave C frees the single-flight + owner go. Closure
      proof: `replay --audit-excluded <4 run IDs>` reports 0 poisoned.
- [ ] T14 Unknown-cleanup loop per wave (<0.5% unknown), armed
      automatically on a wave-gate FAIL.
- [~] T15 Census report (journal-derived) — skeleton written:
      reports/census/REPORT.md (17-run inventory, excluded runs pinned,
      supersession accounting, wave tallies). Final whole-census
      distribution section lands after wave C + poisonfix close.

## Whole-TLD coverage (the "all active .de/.ch/.uk" goal)

- [~] .uk (~10.5M): Nominet zone access — draft ready for owner review
      (docs/zone-data/nominet-uk.md). Sending the email + MiTek ID
      verification = OWNER action; scanning follows the Files.com
      credentials.
- [ ] Scan `zone-ch.txt` (2,660,632 unique apexes, sha256 in
      reports/census/zone-data/MANIFEST.sha256) when the Tranco waves
      free the single-flight; gate params per the 00-42 plan (R3).

## Code & library

- [ ] go-cqrs-lite projectionhost: ship a settled/catch-up API (e.g.
      `Host.WaitUntilSettled`) so consumers don't hand-roll poll loops —
      this repo's interim is `scanstore.WaitProjectionsSettled`
      (projections.go), pinned by
      `TestWaitProjectionsSettledMakesUnfoldedJournalVisible`. Library
      work; post-census (the journal is live mid-wave).
      (repo: go-cqrs-lite)
- [ ] scanstore audit tails (docs/status/2026-11-11_17-43 §f5/§f19):
      read `sqliteengine/v4` `MapUpdate` source and confirm the
      nil-prev upsert directly (closing the inferred-from-tests loop),
      and pin the Recorder sequential-dispatch (journal-order) contract
      with an explicit test.
- [ ] go-cqrs-lite patch sweep when the census frees the journal
      (system v4.18 / metaengine v4.23 baseline): check for bugfix
      releases; re-verify the `foldByEvent` last-fold-wins gotcha after
      any bump (go-ecosystem-upgrade owns the sweep).
      (repo: go-cqrs-lite)
- [ ] STACK.md row for go-codec — a declared direct dependency since
      cborsanitize.go (the CBOR UTF-8 repair); stack-selection owns the
      decision record.
- [ ] asciinema demo / GIF for the README (plan v3 E20).
- [ ] io-history-ratchet: `--profile-io` investigation (56 files vs
      median 10 — a memo layer may have stopped eliminating I/O).
- [ ] `incoming/outgoing-num-tcp` contention measurement post-B
      (are 150 ever contended at w8? → reports/census/unbound/NOTES.md).
- [ ] Poller: per-rank-band unknown-rate expectations, calibrated on
      wave B's pass-1 (~2.8% band vs wave A's 1.53%) + bound the
      "none"-class share per band.
- [ ] Quiet the stale LSP golangci diagnostics (IDE noise vs
      `golangci-lint run` = 0 truth; advisory-only per AGENTS.md).

## Operational

- [ ] Verify the crt.sh timer run (2026-11-14 ~04:37 + jitter) lands
      in-repo — the first legit run since the wrong-root fix; the
      plausibility guard must accept a ≥100k-row .de pull
      (reports/census/zone-data/).
- [ ] Post-B verification bundle (00-42 plan Q4): auto-snapshots flowing
      into reports/census/snapshots/ post-B; single-flight pgrep holds
      through C + poisonfix; one full `-race ./...` on the settled HEAD
      (single lane; wave B running is fine); CI green on every pushed
      HEAD through the chain (release law).
- [ ] BuildFlow#56 fleet fix (owner approval gate): the installed
      buildflow binary still fails valid commits (2026-11-14: its
      embedded older toolchain demands a `go mod tidy` that the repo's
      toolchain proves is a no-op) — the pre-commit hook stays
      bypassed (--no-verify + direct gate runs) until rebuilt. Verify
      via direct gates.
- [~] Audit upkeep (2026-11-10 library audit): citation verification
      green; REMAINING: re-shoot the docs/review/libs index render
      proof and re-audit the go-cqrs-lite/cqrs-htmx verdicts (their
      event-store work may flip the premise).
- [ ] Owner-decision package (00-42 plan R5): ONE doc, one ask, with
      options + recommendation each — attended vs auto-chained wave-C
      launch (open since 21-15 §g1), GPG signing policy for agent
      commits, BuildFlow#56 rebuild approval, Nominet email send,
      poisonfix-25 launch go.
- [ ] Kith CRM delta rescans: confirm the CRM side can retire its own
      diff logic in favor of `replay --since` deltas (T16 landed; the
      plan pointer says this retires at T16). (repo: crm)
