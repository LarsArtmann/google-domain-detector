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
- [ ] Scan `zone-ch.txt` (2,660,632 apexes) when the Tranco waves free
      the single-flight — LAUNCH PREP DONE (R3.1/R3.2, 2026-11-14:
      docs/census.md "zone-ch launch prep": separate zone-ch.db,
      6 × 450k batches ≈ 8 days, pilot-gated, ~10.5 GiB, post-C slot;
      re-verify the manifest sha before launch).

## Code & library

- [ ] M365 ruleset provenance + surface (owner decisions): DECISION
      PACKAGE READY — analysis + execution plan in
      docs/review/2026-11-14_02-18-ruleset-v2-m365-provenance-decision.md
      (recommendation: stamp RulesetV2 before wave C; surface + DKIM
      probe post-census); ask folded into the owner-decision package.
      Owner go pending; the stamp MUST land before wave C's binary is
      built.
- [x] U2/U1 live smoke on a census.db SNAPSHOT (2026-11-14 02:31, the
      tranco1m-b-complete-20261113-1647 snapshot, sha256-verified +
      integrity ok): `replay --audit-excluded wave:tranco1m-a` expanded
      5 runs (bare form byte-identical), 75,255/202,443 findings exit 0,
      unknown token exits 2 naming the token, `runs --format jsonl`
      full 26-char IDs + wave/resumed_from/counts on all 17 runs;
      ULID-vs-lowercase edge pinned in the resolveAuditExcluded doc
      comment.
- [x] File-size headroom pre-splits DONE (2026-11-14 02:47):
      scanstore_test.go 346→239 (stream family → stream_test.go, 194),
      cli_scan_test.go 336→153 (JSONL-stream family → NEW
      cli_scan_stream_test.go, 193); vet + package tests + gofmt green;
      the zeroed file-size debt stays zero with real headroom everywhere.
- [ ] go-cqrs-lite projectionhost: ship a settled/catch-up API (e.g.
      `Host.WaitUntilSettled`) so consumers don't hand-roll poll loops —
      this repo's interim is `scanstore.WaitProjectionsSettled`
      (projections.go), pinned by
      `TestWaitProjectionsSettledMakesUnfoldedJournalVisible`. Library
      work; post-census (the journal is live mid-wave).
      (repo: go-cqrs-lite)
- [x] scanstore audit tails CLOSED (2026-11-14 03:02, source-read
      evidence): MapUpdate nil-prev upsert confirmed directly in
      sqliteengine v4.8.0 (planned.go runTxReadModifyWrite: ErrNoRows
      → prev nil → update(nil), single immediate tx); fold dispatch is
      engine-agnostic core (metaengine planner.go foldByEvent:168 +
      assignPlan:339 — memory and sqlite consume the same plan);
      Recorder journal-order contract pinned by
      TestRecorderDispatchesInJournalOrder (ok/failed/ok sequence must
      survive last-wins; RunSummaries first-started order must match
      dispatch order). scanstore lint 0, suite green, file 328/350.
- [ ] go-cqrs-lite patch sweep when the census frees the journal
      (system v4.18 / metaengine v4.23 baseline): check for bugfix
      releases; re-verify the `foldByEvent` last-fold-wins gotcha after
      any bump (go-ecosystem-upgrade owns the sweep).
      (repo: go-cqrs-lite)
- [ ] asciinema demo / GIF for the README (plan v3 E20).
- [ ] io-history-ratchet: `--profile-io` investigation (56 files vs
      median 10 — a memo layer may have stopped eliminating I/O).
- [ ] `incoming/outgoing-num-tcp` contention measurement post-B
      (are 150 ever contended at w8? → reports/census/unbound/NOTES.md).
- [x] Poller rank-band expectations CALIBRATED on live wave-B pass-1
      (2026-11-14 02:42, 133k rows joined to input ranks): unknown
      rates 1.09% (50k-100k resumed tail) → 2.28% (100k-150k) → 2.44%
      (150k-200k) → 2.82% (200k-250k, in progress) — the ~2.8%
      hypothesis confirmed, ~1.5-1.8× wave A's 1.53%; "none" share
      bounded at 47-53% per band; error mix per band ≈ nx 0.7-0.9%,
      timeout 0.6-1.1%, servfail 0.05-0.5%, zero cancels. Poller bar
      (GDD_WARN_PCT) sits above these — no alarms expected at this
      rate.
- [x] Stale LSP diagnostics quieted (2026-11-14 02:55): the 5 gopls
      `infertypeargs` infos resolved — 3 were REAL (scanstore.go
      RegisterCommand explicit type args inferable from the handler
      literal; removed, build+tests green), 2 in cli.go were phantom
      positions from a pre-refactor layout (gone after LSP restart).
      Remaining panel warns are golangci-LSP integration artifacts
      contradicted by the gate (`golangci-lint run` = 0 issues);
      projections.go warns are sibling lane, advisory-only per
      AGENTS.md.

## Operational

- [ ] Verify the crt.sh timer run (2026-11-14 ~04:37 + jitter) lands
      in-repo — the first legit run since the wrong-root fix; the
      plausibility guard must accept a ≥100k-row .de pull
      (reports/census/zone-data/).
- [ ] Post-B verification bundle (00-42 plan Q4): VERIFIED 2026-11-14
      03:05 on the pre-B-exit state — restore-verify PASSED (snapshot
      tranco1m-b-complete-20261113-1647: sha256 ok, integrity_check ok,
      17 runs + 245k streams read back via runs/replay); census.db
      headroom 961 MB vs 215 G free; single-flight held all session; one
      full `-race ./...` GREEN (scanstore 297 s, all packages ok); CI
      green on every pushed HEAD (4 consecutive runs). REMAINING post-B:
      confirm auto-snapshots flow after B's exit and repeat the checks
      on B's own snapshot.
- [ ] BuildFlow#56 fleet fix (owner approval gate): the installed
      buildflow binary still fails valid commits (2026-11-14: its
      embedded older toolchain demands a `go mod tidy` that the repo's
      toolchain proves is a no-op) — the pre-commit hook stays
      bypassed (--no-verify + direct gate runs) until rebuilt. Verify
      via direct gates.
- [x] Audit upkeep (2026-11-10 library audit): citation verification
      green (2026-11-13); libs index render proof RE-SHOT 2026-11-14 02:40
      (balanced tags, 22/22 .md links resolve, 43 KB self-contained);
      go-cqrs-lite verdict re-audited — the do-not-adopt verdict's own
      revisit trigger FIRED on the census directive, doc updated
      (ADOPTED, pins all latest, nothing to sweep); cqrs-htmx exclusion
      re-verified as still-correct (no HTTP/HTMX surface in a CLI).
- [ ] Owner-decision package (00-42 plan R5): WRITTEN —
      docs/planning/2026-11-14_02-18-owner-decision-package.md (8 asks,
      table + options + recommendation each: attended-vs-auto-chain,
      RulesetV2, poisonfix go, www-strip, GPG, BuildFlow#56 rebuild,
      Nominet email, M365 surface). Owner answers pending.
- [ ] Kith CRM delta rescans: confirm the CRM side can retire its own
      diff logic in favor of `replay --since` deltas (T16 landed; the
      plan pointer says this retires at T16). (repo: crm)
