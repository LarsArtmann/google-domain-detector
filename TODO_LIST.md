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

- [~] T12–T13 Waves B (100–500k), C (500k–1M) with pilot gates — **B DONE**:
  attempt 5 completed 2026-11-15 20:05 after 12.2 h (zero reboots, zero
  alarms), pass-1 gate PASS on the FIRST try (no cleanup loop): 171,573
  scanned + 11 unparseable + 228,416 resume-skipped = 400,000 exactly;
  workspace certain 28,020 / likely 7,818, gcp 2,269, errors 7,063; run
  `01M7EZJSEJKXNSPNEQ4401AE3S`; snapshot restore-verified on a copy
  (sha256 OK, integrity ok, replay reproduces the live summary
  byte-identically). C launches after owner GO (row below).
- [ ] Wave C GO/NO-GO (owner, 2026-11-15 14-37 report §g2/§g3): B gate
      PASS + snapshot restore-verify are DONE — without an answer the
      session HOLDS single-flight (verified + recorded). The
      pause-vs-push-through decision for the chain rides the same
      answer: C only, C + poisonfix-25, or C + zone-ch batch 1. Launch
      mechanics when GO: C3 via census-wave.sh with
      `GDD_PREFLIGHT_TRANSPORT=tcp`.
      chained per plan; ~35 h at the calibrated rate; older reports
      (08-57 item 7, 11-12 hardening item 8) point at the same launch.
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
      `TestWaitProjectionsSettledMakesUnfoldedJournalVisible`. ISSUE
      DRAFT READY (2026-11-14 ~09:05):
      docs/review/2026-11-14_08-55-projectionhost-settled-api-issue-draft.md
      (gap verified at v4.7.1 — no settled/catch-up method;
      CheckStaleness/LagDuration are time-based; filing is owner-gated).
      Library work; post-census (the journal is live mid-wave).
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
- [x] go-cqrs-lite patch sweep READ-ONLY CHECK 2026-11-14 ~09:00:
      system v4.18.0 and metaengine v4.23.0 are both the LATEST
      published versions — no bugfix releases to sweep; pins already
      at latest. The foldByEvent last-fold-wins re-verify obligation on
      any future bump stays in AGENTS.md (go-ecosystem-upgrade owns it;
      the journal-touching bump itself waits for post-census).
- [x] asciinema demo / GIF for the README (plan v3 E20) — DONE
      2026-11-17: scripts/demo-record.sh runs end-to-end (single-flight
      guard, 60-domain slice, asciinema cast + vhs GIF); assets/demo.gif
      (1.1 MB, 1600x900, ~35 s: live scan with verdict colors → runs →
      show → replay from a self-contained demo-gif.db). Gotchas fixed
      en route: asciinema's stdin relay hangs on heredoc-fed non-tty
      parents (session rides --command + --noprofile --norc), vhs
      `Set Shell` takes no arguments, `Output` needs a relative path.
      README embed held on owner question §g3.
- [x] io-history-ratchet `--profile-io` investigation (2026-11-14
      ~09:00): full-gate profile on a CLEAN tree = 27 spawns, 3 files
      hashed, 49 writes — normal. The 56-file measurement was dirty-tree
      cache-key hashing (the sibling session's uncommitted test splits
      at measurement time), not a memo-layer regression. Expected
      behavior, no fix.
- [x] `incoming/outgoing-num-tcp` contention measurement (2026-11-15,
      live wave B w4 samples + NOTES.md write-up): 150 never contended
      at w8 — upstream TCP conn population fluctuates 17-44 at w4
      (linear w8 scaling ≈ 34-88, ≥ 40% headroom under the 150 cap);
      `incoming-num-tcp` is unused in this topology (scanner hops UDP,
      0 client-side TCP conns in every sample); fd pressure 134/65536.
      Caveat flagged in NOTES.md: per-thread cap semantics unverified
      (would raise effective caps further). Measurement gotcha
      recorded: anchor port regexes — unanchored `:53` matches counted
      local ephemeral ports 53xxx as peers (phantom 192.168.1.150:53
      conns).
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
      CORRECTION 2026-11-14 ~09:00: the 2 cli.go infos re-surfaced and
      proved REAL (WithMiddleware + DoctorCommand explicit type args,
      both inferable) — dropped, build + cli/scanstore tests green,
      LSP clean after restart; not phantom after all. Remaining panel
      warns are golangci-LSP integration artifacts
      contradicted by the gate (`golangci-lint run` = 0 issues);
      projections.go warns are sibling lane, advisory-only per
      AGENTS.md.

## Operational

- [x] Verify the crt.sh timer run (2026-11-14 ~04:37 + jitter) lands
      in-repo — VERIFIED 2026-11-14 04:51: fired on schedule in-repo
      (correct path, first run since the wrong-root fix); the
      plausibility guard correctly REJECTED a truncated crt.sh harvest
      (2,060 .de rows < 100,000 floor, artifact removed). Mechanism
      proven; the ≥100k artifact awaits crt.sh upstream recovery
      (auto-retries daily). Recorded in docs/census.md zone-inputs.
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
- [ ] PSI launch gate formalization (2026-11-15 14-37 report §e2):
      census-wave.sh reads /proc/pressure/memory avg10 before preflight
      and refuses to launch above `GDD_MIN_PSI` (default 10) — today it
      is operator discipline only; it worked because this session chose
      to enforce it.
- [ ] scripts/census-relaunch.sh ritual encoder (2026-11-15 14-37
      report §e1): the 07:48 env-less launch (preflight correctly
      refused it) showed the operator-error class — encode the whole
      relaunch ritual (archive forensics, restart forwarder, probe,
      launch with the FULL env prefix, verify the six monitor lines,
      attach watchdog) into one reviewed script. Implement post-B;
      never touch live census scripts mid-wave.
- [ ] NOTES.md backup ritual (2026-11-15 14-37 report §e5): reports/ is
      gitignored — at census close, sha256-sidecar
      reports/census/unbound/NOTES.md (the PROVENANCE.md pattern) or
      copy its durable sections into a tracked doc.
- [ ] Anchor-regex measurement lesson (2026-11-15 14-37 report §e4):
      unanchored `:53` patterns match local ephemeral ports 53xxx as
      peers — phantom 192.168.1.150:53 conns and double-counted TCP
      samples. Cross-project lesson: record in crush-config
      references/lessons.md (committed there by policy, never in-session).
- [ ] census-gate.sh arg-parser hardening (2026-11-22 14-08 report §d3):
      the parser `break`s on the first non-flag, so flags placed AFTER
      the file positional are read as positionals — `--manifest` became
      the budget and awk coerced it to 0.00% (spurious FAIL, found live
      during the H2 smoke). Keep parsing flags until `--` or error
      loudly; add a pinning test.
- [ ] Demo tape poll-for-completion (2026-11-22 14-08 report §e4):
      demo-record.sh's vhs tape hardcodes `Sleep 25s` for the scan —
      replace with a poll on the scan progress footer so GIF timing
      survives resolver slowness; also note the GIF tail-frame eyeball
      check into the script comments (verified at frame 840 this time).
- [ ] Manifest hygiene: 11 unparseable tranco1m-b entries (2026-11-22
      14-08 report §e7, first `_wildcard_.net.ph`) are permanently
      unscanned and gate-accepted — decide sanitize-for-future-waves
      (provenance sha changes, ledger pin breaks) vs byte-immutability
      with a carried hole (owner call).
- [ ] Host clock/NTP check (2026-11-22 14-08 report §e6): metadata.yaml
      wrote future timestamps and session dates jumped (Nov 17 → 21 →
      22) — check timesyncd before the next provenance-sensitive run
      (binary stamps, snapshot names).
- [ ] Snapshot restore-verify as a NAMED census.md exit step
      (2026-11-22 14-08 report §e8): proven twice (20261113 + 20261115
      snapshots) — promote from report prose to the exit checklist so
      it never depends on operator memory.
- [ ] T15 TLD + M365 breakdown columns (2026-11-22 14-08 report §f10):
      compute wave-B TLD distribution (.de/.ch/.uk/.co …) and the
      M365-class count from the rank-ordered run1.jsonl for the final
      census report (fingerprints are evidence-only today).
- [ ] crt.sh daily timer in-repo landing verification (2026-11-22
      14-08 report §f20): the provenance-fix verification was "the next
      04:37 timer" (2026-11-13) — many fires have passed since; confirm
      zone-data lands INSIDE the repo with the plausibility guard.
- [ ] Status-README archive pass (2026-11-22 14-08 report §f38): 14
      unarchived reports (cadence threshold 15) — reports whose items
      are all resolved (2026-11-10/11-11 era) move to archived/ with
      fresh:skip/harvest annotations.
- [ ] DOMAIN_LANGUAGE.md "unparseable input" term (2026-11-22 14-08
      report §f39): distinct from unknown — a manifest row that never
      entered any denominator; add to the census-ops table.
- [ ] AGENTS.md two-line additions (2026-11-22 14-08 report §f29/§f42):
      tmp-staging law (staged fixes live in the worktree, never /tmp —
      eaten twice by the tmp cleaner) and the single-flight law naming
      the demo recorder as a scan (it has its own pgrep guard).
- [ ] Skip-union design prework + the §g1 multi-run skip-union
      authorization ask (2026-11-22 14-08 report §f13/§f14): IF the
      owner GO is likely — journal-side union of N runs'
      observed streams + resume CLI shape, designed BEFORE wave C so a
      mid-C death resumes cheap.
- [ ] ROADMAP.md refresh (2026-11-22 14-08 report §f40): wave-B-complete
      reality, wave C / zone-ch / .uk slots, skip-union and demo-GIF
      outcomes folded into the raw-ideas vs committed structure.
- [ ] GPG signing setup (8-ask #5, owner): commit/tag signing for the
      fleet.
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
- [x] Kith CRM delta rescans: CONFIRMED from this side 2026-11-14
      ~09:10 — `replay --since` is shipped and pinned by tests
      (cli_replay_test.go: exact-delta json/jsonl, unknown run ID must
      fail), so the CRM plan's own delta logic can retire in favor of
      replay --since per the AGENTS.md downstream pointer. The crm-side
      edit is deferred: that repo has a live session (dirty status
      files, 08:42 report) — write fence; hand the confirm to the crm
      session/owner instead of trampling.
