# CI-green restored, verification wall closed, audit tails at source — session a–g

**Session:** 2026-11-14 02:05 → 03:20 CET (resumed from the 02-05
closeout with the owner's "GET SHIT DONE — the whole TODO list"
directive; the 02-46 docs-health pass ran CONCURRENTLY in the first
half and its report is integrated, not duplicated). **HEAD `45ddfc2`,
pushed to both remotes, CI green on every pushed HEAD** (4 consecutive
green runs after the red streak documented in §d1). Wave B RUNNING
throughout: 135,980 scanned at 02:54 (~3.8/s), 0 watchdog alarms,
single-flight held (exactly one `gdd scan` at every check).

## a) FULLY DONE (all gate-verified)

1. **CI red → green.** Session-start `gh run list` showed THREE
   consecutive red runs on master. Root cause: my own `audit.go` from
   the U2 wave-names lane carried 2 mid-flight lint findings
   (`runReplayAudit` cyclop 15>12, ineffassign `isWave`) that shipped
   because only the file-size + structure gate steps had been run
   individually. A concurrent session had already committed the fix
   (`07be69c`) — but 10 commits sat unpushed, so CI never saw it.
   Verified local lint = 0 (golangci 2.15.0, same as CI), pushed both
   remotes, CI green since.
2. **FULL buildflow gate end-to-end: PASS (exit 0), twice** (fresh run
   + cached re-run for exit-code confirmation). Remaining gate findings
   are all advisory warns in documented non-fix territory
   (go-version-auto-configure vs the cqrs floor, vulnix nixpkgs-level
   CVEs, lychee 403 on an external site, shellcheck on the FROZEN
   census scripts, inline-vendorHash style suggestion).
3. **`buildflow format`: no-op** — formatter fingerprint drift already
   resolved; tree byte-identical after the run.
4. **vendorHash verified TRUE by `nix build`** (not just the gate): the
   hermetic build succeeds and reproduces; the stale-vendorHash gate
   warning is noise. `nix-hash-fix --fix` needed no action.
5. **U2/U1 live smoke on a REAL census.db SNAPSHOT** (never the live
   db) — `tranco1m-b-complete-20261113-1647.tar.gz`, sha256-verified:
   `replay --audit-excluded wave:tranco1m-a` expanded 5 runs
   oldest-first (bare `tranco1m-a` byte-identical), reported
   **75,255 of 202,443 domains** last observed by an excluded run,
   exit 0 with per-run counts on stderr (all 75,255 from the resumed
   97k run — earlier A runs fully superseded, exactly as the
   supersession model predicts); unknown token exits **2** naming the
   token; `runs --format jsonl` full 26-char IDs + wave/resumed_from/
   counts on all 17 runs; ULID-vs-lowercase-wave-name non-collision
   pinned in the `resolveAuditExcluded` doc comment.
6. **Snapshot restore-verify PASSED** (part of the post-B bundle,
   pulled forward): sha256 ok → tar extract → `integrity_check ok` →
   17 runs + 245,179 streams read back through `runs` + replay.
7. **Full `-race ./...` GREEN once on the settled HEAD** (single lane;
   wave B running is fine per the row): every package ok, scanstore
   297 s under race, CI-matching `-timeout 20m`.
8. **File-size headroom pre-splits** — scanstore_test.go 346→**239**
   (stream family → stream_test.go, 194), cli_scan_test.go 336→**153**
   (JSONL-stream family → NEW cli_scan_stream_test.go, 193); vet +
   package tests + gofmt + `golangci-lint run` 0 issues; the zeroed
   file-size debt stays zero with ~100 lines headroom on both.
9. **LSP ghost diagnostics resolved** — 5 `infertypeargs` infos: 3 were
   REAL (`scanstore.go` RegisterCommand explicit type args inferable
   from the handler literal — removed, build+tests green), 2 in cli.go
   were phantom positions from a pre-refactor layout (vanished after
   LSP restart). Remaining panel warns are golangci-integration
   artifacts contradicted by the gate's 0 issues.
10. **Scanstore audit tails CLOSED at source** (the 17-43 §f5/§f12/§f19
    row): (a) MapUpdate nil-prev upsert confirmed by READING
    sqliteengine v4.8.0 (`planned.go` `runTxReadModifyWrite`:
    ErrNoRows → prev nil → `update(nil)`, one immediate tx); (b) fold
    dispatch verified engine-agnostic (metaengine `planner.go`
    `foldByEvent`:168 + `assignPlan`:339 — memory and sqlite consume
    the same plan); (c) Recorder journal-order contract pinned by NEW
    `TestRecorderDispatchesInJournalOrder` (an ok/failed/ok dispatch
    sequence must survive last-wins; RunSummaries first-started order
    must match dispatch order).
11. **RulesetV2 decision note** (docs/review/2026-11-14_02-18): the
    ruleset contract's own MUST ("new evidence kinds") is violated by
    running wave C under V1 after the M365 fingerprints; recommendation
    = stamp `v2-2026-11-14` before wave C's binary exists; execution
    plan ≈10 min; journal append-only, no migration; Google-side
    comparability preserved across all waves (test-pinned invariance).
12. **Owner-decision package WRITTEN** (docs/planning/
    2026-11-14_02-18): 8 asks in one table (attended-vs-auto-chain,
    RulesetV2, poisonfix go, www-strip rescan slot, GPG, BuildFlow#56
    rebuild, Nominet email, M365 surface) with options +
    recommendation each. Ask #4 CORRECTED after the concurrent session
    revealed the www-strip POLICY was already decided (census.md
    2026-11-12: normalize + scan) — the ask is now only the rescan
    slot.
13. **Audit upkeep (Q3) closed**: libs index render proof re-shot
    (balanced tags, 22/22 .md links resolve, 43 KB self-contained —
    structural proof; no browser available for pixels); go-cqrs-lite
    verdict re-audited — its own revisit trigger FIRED (adoption is
    load-bearing), doc updated with ADOPTED banner + all pins verified
    LATEST (system v4.18.0, sqliteengine v4.8.0, projectionhost
    v4.7.1, event v4.15.2, command v4.14.1, projectionadapter v4.6.2 —
    nothing to sweep); cqrs-htmx exclusion re-verified as still
    correct.
14. **DOMAIN_LANGUAGE.md authored** (the doc set's flagged missing
    must-have): ubiquitous language for the census domain — core nouns,
    event-sourcing terms, census-ops terms, and the split-brain list —
    every evidence kind and platform verified against
    `classify.go` constants (initial draft had 2 wrong enum values;
    corrected against source before commit).
15. **Rank-band calibration** from LIVE wave-B pass-1 data (133k rows
    joined to input ranks): unknown rates 1.09% (50k-100k resumed
    tail) → 2.28% → 2.44% → 2.82% (in progress) — the ~2.8% hypothesis
    confirmed (~1.5-1.8× wave A's 1.53%); "none" share bounded 47-53%
    per band; zero cancels. Poller bar sits above these.
16. **zone-ch launch prep (R3.1/R3.2)** into docs/census.md: separate
    `zone-ch.db` (keeps tranco replays fast), 6 × 450k batches ≈ 8
    days at the calibrated rate, pilot-gate the first 10k into a
    throwaway store (raw zone lists will run HIGHER unknowns than
    tranco's ranked bands), ~10.5 GiB projected vs 215 G free, input
    sha256 re-verified (`f845b471…`, 2,660,632 apexes).
17. **STACK.md go-codec row + decision-log entry** (declared direct dep
    since cborsanitize; "why this, not that" = hand-rolled byte
    rewriting can't speak CBOR framing); ROADMAP gained the CRM-delta
    retirement bullet (enrichment/dedup bullets already existed).
18. **io-surface gate finding fixed**: ranges cache read now opens one
    handle (`os.Open` + `ReadAll` + `Close` both checked) instead of
    Stat+ReadFile TOCTOU/double-syscall; ranges tests + lint 0.
19. **TODO_LIST truth pass**: struck with evidence — U2/U1 smoke,
    file-size pre-splits, audit upkeep, scanstore tails, rank-band,
    STACK row, LSP noise, M365 decision (pointer), owner package
    (written); annotated the post-B bundle row (4/5 components
    verified pre-exit).
20. **Push hygiene**: every unit pushed to BOTH remotes; the daemon
    won one commit race (bundle of my 4 files) and the message-only
    amend (`--no-verify`, tree byte-identical — the policy's exact
    condition) gave it a real message.

## b) PARTIALLY DONE

1. **C1 census watch** — heartbeats at 02:24/02:54 (135,980 scanned,
   ~3.8/s, 0 alarms), single-flight `pgrep` = exactly 1 at every
   check; jsonl growth is the live signal per the documented
   hierarchy. The watch CONTINUES (it is wall-clock, not work) until
   B's ~14:45-16:00 exit.
2. **crt.sh 04:37 timer verification** — the FIRST legit post-fix run;
   time-gated, fires in ~80 min at report time. Everything it needs is
   in place (wrapper plausibility guard branch-tested, wrong-root fix
   landed 2026-11-13).
3. **Post-B verification bundle** — 4/5 components verified early (see
   a6/a7 + headroom + CI); remaining is inherently post-B: confirm
   auto-snapshots flow after B's exit and re-verify on B's own
   snapshot.
4. **Gate-warn hygiene** — shellcheck SC2034 (`errors` unused in
   census-gate.sh) and the frozen census-wave.sh findings: DELIBERATELY
   deferred — the gate script must not change before wave B's critical
   gate run (~15:00), and census-wave.sh has a live executing instance
   (inode law). Queued post-B.
5. **This report** — written at 03:18 per the ritual; the closing
   census-closeout report belongs to the post-C5 ritual, not this one.

## c) NOT STARTED (correctly gated, not dropped)

- **Wave C launch, RulesetV2 stamp, poisonfix-25** — owner gates
  (package asks #1-#3); RulesetV2 MUST land before wave C's binary is
  built or the provenance split becomes permanent for the .ch/tranco
  comparisons.
- **Nominet email send, GPG signing, BuildFlow#56 rebuild** — owner
  actions (asks #5-#7).
- **asciinema demo / GIF (E20)** — needs a scan slot (single-flight);
  post-B.
- **TCP contention measurement (`incoming/outgoing-num-tcp`)** — post-B
  data by definition.
- **DOMAIN_LANGUAGE docs-gates integration** — next full gate will
  validate the new file against freshness/annotate rules (the 03:00
  gate ran before the glossary landed).

## d) TOTALLY FUCKED UP

1. **The CI red streak is mine.** The U2 feature shipped with 2 lint
   findings because only two gate steps were run individually (the
   02-05 handoff's own "run the FULL gate" step was still pending) —
   every push from ~00:30 to 01:53 landed red on master. The concurrent
   session's fix then sat unpushed ~20 min while I initially diagnosed
   "the findings exist" instead of "the fix is committed but unseen".
   Lesson recorded in §e1/e2.
2. **Exit-code test masked by a pipe** — the unknown-token check first
   ran as `cmd | tail; echo $?`, which reports tail's exit (0). Caught
   on inspection, re-run without the pipe: exit 2, correct. A wasted
   round-trip and a near-false-pass on the feature's key error path.
3. **`sha256sum -c` run from /tmp** against a checksum file with
   repo-relative paths → one FAILED open, one wasted round-trip.
   Test the literal path string a script embeds (the crt.sh lesson,
   re-learned at small scale).
4. **Three edit refusals from daemon sweeps mid-edit** (TODO_LIST ×2,
   owner package ×1) — the auto-commit daemon makes read-before-edit
   stale within seconds. Adapted with python file-splice edits after
   re-reads, but the round-trips were avoidable by committing units
   immediately.
5. **My commit lost the race with the daemon** (`git add -A && git
   commit` arrived after the sweep) — the pre-commit "failure" was an
   empty commit. Recovered cleanly via the policy's message-only amend
   (tree byte-identical), but the sequencing (commit BEFORE long gates)
   would have avoided it.
6. **golines finding on my own new test** — declared the scanstore
   tails done, then the linter flagged an unwrapped long line. Lint
   before "done", every time.

## e) WHAT WE SHOULD IMPROVE

1. **No feature lane is done until the FULL gate ran** — per-step
   verification is how the audit.go findings reached master. The gate
   is one command; run it before every push, not after the red.
2. **Diagnose "unpushed" before "unfixed"** — `git log
   origin/master..HEAD` + `gh run view <sha>` is a 10-second check that
   would have halved the CI-red diagnosis.
3. **Never capture exit codes through pipes** — `$?` after `|` is the
   pipe's last stage. Use PIPESTATUS or drop the pipe in verification
   commands.
4. **In a live-daemon repo, commit each unit immediately** — small
   named commits beat both junk bundles and mid-edit staleness
   refusals (the 02-46 session reached the same conclusion
   independently).
5. **AGENTS.md candidate (blocked at 219/220 budget):** "sha256sum -c
   checksum files use repo-relative paths — run from the repo root";
   park for the next AGENTS diet.
6. **LSP restart is the ground-truth ritual** for ghost diagnostics —
   half of tonight's "findings" evaporated on restart; verify before
   editing.

## f) UP TO 50 THINGS TO GET DONE NEXT

**Census critical path (single-flight, wall-clock, in order):**

1. Heartbeat/single-flight ritual every ~30 min until B exits
   (~14:45-16:00); jsonl growth = primary signal.
2. crt.sh 04:37 timer verify: wrapper ran in-repo, plausibility guard
   accepted ≥100k .de rows, artifacts under
   reports/census/zone-data/, monitor.log merged.
3. On B exit: wrapper stderr summary → run-id file.
4. Verify B's auto-gate PASS (`--manifest --expect-skip 96215`).
5. If gate FAIL: T14 cleanup loop + explicit-resume reconcile (never
   `--resume latest`).
6. Owner go (ask #2): stamp RulesetV2 (`v2-2026-11-14`, ~10 min) —
   BEFORE wave C's binary is built.
7. Owner go (ask #1): wave-C launch mode (attended default).
8. Launch wave C (`tranco1m-c`, fresh pass, w4/10s, 5354, TCP
   preflight, PROVENANCE-stamped binary incl. the V2 stamp if
   approved).
9. Attach a fresh watchdog to wave C.
10. Verify post-B auto-snapshots flowed; re-run the bundle checks on
    B's own snapshot (the 1/5 remainder of §b3).
11. Owner go (ask #3): poisonfix-25 (fresh binary, PROVENANCE stamp,
    25-domain list).
12. Closure proof: `replay --audit-excluded <4 run IDs>` → 0 poisoned.
13. Census.db headroom re-check for wave C + zone-ch.
14. T15 final whole-census distribution in reports/census/REPORT.md
    (journal-derived; label which runs saw Microsoft evidence per the
    ruleset question).
15. Closing status report + index row + push (the census closeout
    ritual).
16. `pgrep 'gdd[^ ]* scan'` = exactly ONE through C + poisonfix.

**Code (unblocked):**

17. Full buildflow gate once more after the glossary landed (docs-gates
    freshness/annotate vs DOMAIN_LANGUAGE.md).
    → DONE 2026-11-14 08:5x: fresh-cache full gate PASS, exit 0 confirmed
    via cached replay (BUILDFLOW_NO_RESULT_CACHE omitted on the confirm
    run); remaining warns all documented non-fixes.
18. shellcheck SC2034 in census-gate.sh (post-B only — gate frozen
    until then; new-inode install).
19. `buildflow -s go-auto-upgrade --fix` evaluation on the json-v1
    audit findings (provenance_test.go) — decide fix vs documented
    non-fix.
    → DONE 2026-11-14 09:0x: FIX applied manually — 4 test files
    (audit_test, cli_scan_test, cli_scan_stream_test, provenance_test)
    moved to encoding/json/v2; sinks are string-only reads and the
    flagged time.Duration fixture field is never serialized, so the
    tool's conservative skip didn't apply. Packages green, gofmt clean,
    json-v1 findings 26→0. The remaining 21 suggest-lo-* findings are a
    DOCUMENTED NON-FIX: samber/lo is not a go.mod dependency and adding
    it for style sugar is a stack decision, not hygiene.
20. scanstore_test.go at 328/350 — fine now; pre-split before any
    addition >22 lines.
21. go-cqrs-lite patch sweep re-check post-census (pins verified latest
    today; only the foldByEvent re-verify after any future bump).

**Owner-gated (package asks, executable on answer):**

22. Ask #1: attended vs auto-chained wave-C launch.
23. Ask #2: RulesetV2 stamp (execution plan in the decision note).
24. Ask #3: poisonfix-25 go.
25. Ask #4: www-strip rescan-in-C go (policy already decided).
26. Ask #5: GPG signing policy for agent commits.
27. Ask #6: BuildFlow#56 fleet rebuild approval.
28. Ask #7: Nominet .uk email send (draft ready,
    docs/zone-data/nominet-uk.md).
29. Ask #8: M365 surface (render tally vs Counts v2) + DKIM microsoft
    selector probe — park vs plan.

**Post-census / parked (per the round-2 plan):**

30. asciinema demo / GIF for the README (needs a scan slot).
31. TCP contention measurement (`incoming/outgoing-num-tcp` at w8) →
    reports/census/unbound/NOTES.md.
32. zone-ch scan per the R3 prep (separate db, 6 batches, pilot gate).
33. projectionhost settled-API upstream (this repo's
    WaitProjectionsSettled is the reference).
34. Kith CRM delta-retirement confirm (repo: crm).
35. Enrichment join / registrant dedup framing (ROADMAP bullets exist).
36. Lead scoring v1 weights from the final census distribution.
37. Verdict-flip daily digest timer.
38. Journal compaction decision when census.db passes a few GiB.
39. Whole-.uk pipeline when Nominet credentials arrive.
40. .de crt.sh daily cadence continues (row 2 verifies today's).

**Hygiene / process:**

41. AGENTS.md diet candidate list: sha256sum -c relative-path gotcha,
    "no feature lane done until the FULL gate ran".
42. Keep CHANGELOG current through the wave-C window (ruleset stamp
    gets an entry when approved).
43. docs-gates freshness after every status report (this one included).
44. Watch CI on every pushed HEAD through the chain (release law —
    green so far tonight).
45. Re-check FEATURES.md PARTIALLY rows after wave C (the 02-46 pass
    normalized the vocabulary).
46. io-history-ratchet `--profile-io` investigation (56-file offender)
    — research, any time.
47. Consider recording the "none-share per band" bounds in the poller
    config comments (numbers live in TODO_LIST evidence today).
48. Snapshot retention: 3 snapshot tars ≈ 1.9 GB — decide a keep-N
    policy with the owner eventually.
49. crons: confirm the SWITCH .ch 24 h pull and crt.sh timer stay
    healthy through the wave-C window (monitor.log merges).
50. Park: annotate-tool tilde guard (02-46 §e3) — upstream docs-gates
    candidate.

## g) QUESTIONS FOR THE OWNER (cannot self-answer)

1. **RulesetV2 + chain pre-authorization:** the decision note and the
   8-ask package are ready. May I stamp `v2-2026-11-14` NOW (10 min,
   before wave C's binary exists) — and do you pre-authorize the
   B→gate→C→poisonfix chain for when B exits (~15:00), or do you want
   each handoff attended? This is the census's 1% → 51%; everything
   downstream (T15, CRM deltas) derives from it.
2. **On a B gate FAIL:** do I auto-run the T14 cleanup loop and
   reconcile, or hold for your go? (The wrapper arms T14
   automatically; the question is whether unattended failure handling
   is wanted — same territory as ask #1, asked separately because it
   only bites on failure.)
3. **GPG signing for agent commits:** sign everything with a dedicated
   subkey (I'd need the key/passphrase channel you want), keep
   unsigned, or owner-only signing? Current state: all agent commits
   unsigned; the daemon makes per-commit signing a fleet config, not a
   repo config.
