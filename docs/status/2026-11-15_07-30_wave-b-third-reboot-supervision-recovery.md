# Status: wave B through two host reboots — supervision, two recoveries, the doom-loop arithmetic, attempt 5 staged on a PSI gate

**Session:** 2026-11-14 05:37 → 2026-11-15 07:30 CET (continuous except a
host-storm interruption ~02:21 → 07:18) · **Mandate:** execute the
round-2 SUPERB closeout plan (all lanes), supervise wave B, report a–g.

**Session start state:** tree clean at `250bf8f` (both remotes, CI green),
wave-B attempt 2 alive (pid 2327170), single-flight = 1, sibling session
live (fenced dirty test files).

---

## a) FULLY DONE

1. **Round-2 plan reconciliation on resume**: the sibling session had
   landed V2 (U2/U1 live smoke), V4 (scanstore audit tails), V6 (file-size
   splits), D1 (DOMAIN_LANGUAGE.md), D2/D3 (audit upkeep), R1 prep (zone-ch),
   R2 (rank-band calibration), H3 (ranges TOCTOU) and most of V3/H1 while I
   was away — all verified against evidence (TODO rows + commits + CI) and
   marked done in my tracking rather than redone.
2. **H1 residual — the two cli.go `infertypeargs` infos were REAL, not
   phantom**: the struck TODO row claimed they were stale LSP positions;
   they re-surfaced at `WithMiddleware[RootConfig]` (cli.go:120) and
   `DoctorCommand[RootConfig]` (cli.go:203), both inferable from their
   arguments. Dropped, build + cli/scanstore tests green, LSP clean after
   restart, row corrected (not left lying). Commit `2ab65df`.
3. **V5 — crt.sh daily timer verified** (the wrong-root fix's first live
   run): fired 2026-11-14 04:51 on schedule in-repo, and the plausibility
   guard correctly REJECTED a truncated crt.sh harvest (2,060 .de rows <
   100,000 floor, artifact removed, nothing partial kept) — mechanism
   proven end to end; data arrival waits on crt.sh upstream and auto-retries
   daily. Recorded in census.md zone-inputs + TODO struck. Commit `3a76e78`.
4. **R5 — io-history-ratchet investigation closed**: full buildflow run
   with `--profile-io` on a clean tree = 27 spawns / 3 files hashed / 49
   writes (normal). The scary 56-file measurement was dirty-tree cache-key
   hashing (the sibling's uncommitted test splits at measurement time), not
   a memo-layer regression. No fix needed.
5. **R3 lane closed**: go-cqrs-lite patch sweep read-only check (system
   v4.18.0 and metaengine v4.23.0 are the latest published — nothing to
   sweep); **projectionhost settled-API issue draft written and
   verify-gated** (gap confirmed at projectionhost v4.7.1 via
   `go doc -all` — no settled/catch-up method, `CheckStaleness`/
   `LagDuration` are time-based, `RegisterAndWait` registration-only;
   draft at docs/review/2026-11-14_08-55-projectionhost-settled-api-
   issue-draft.md, filing owner-gated); **Kith CRM delta retirement
   confirmed from this side** (`replay --since` shipped and pinned by
   tests; the crm-side edit deferred because that repo had a live session
   — write fence); ROADMAP enrichment/dedup/scoring bullets verified.
6. **V3 re-proven fleet-wide**: full buildflow gate on `3a76e78` —
   **62/62 steps green** including `test-race` (1m29s), `nix-build` (37s),
   `golangci-lint`, `io-surface`, `file-size-check`, `docs-gates`. Verdict
   read from `buildflow explain <run-id>` after discovering redirected
   stdout only carries the banner (the TUI owns the terminal).
7. **Audit-all harvest — 7 old findings dispositioned with real
   evidence**: IPv6 `--resolver` resolved by commit `cb205ba` (3 copies:
   items 41/31/29 across three reports), jq-shim gotcha landed in AGENTS.md
   (item 23), docs-gates cached-vs-direct root-caused to the hook's
   pre-commit build mode (item 28), E16 CI-red item routed to its owning
   repo (item 14), boot-storm load attribution ROUTED then ANSWERED live
   (item 33, see 12). Commit `476a52a`.
8. **Attempt-2 provenance pinned in the wave ledger** (the sibling
   recovery report's §f3 ask): run `01M7ASTM63TVRJ6RXCGR00EYVQ`, 227,126
   domains last-observed per observation_views at 09:56. Commit `d0718d4`.
9. **Wave-B attempt-4 recovery after host reboot #2** (reboot ~15:3x
   2026-11-14, memory storm, killed attempt 3 at 71,171 scanned with no
   drain): forensics archived first, forwarder restarted fresh (unbound-tcp),
   **resume target switched to attempt 2's run id** — legal explicit-id
   resume, skip attribution verified journal-side in
   `LoadResumeSource` (p.RunID == source; supersession-independent), gate
   expect-skip 228,416, workload 171,584 ≈ 12.9 h vs the stale target's
   303,785 ≈ 21.6 h. Commit `807585c`. Attempt 4 then ran **9.5 h healthy
   (15:54 → 02:24)** at ~3.7-4.0/s through a load-1042 fleet storm,
   reaching 140,981 scanned.
10. **Watchdog re-attached** to attempt 4 (it dies with the host on every
    reboot; the wrapper does not restart it) — zero alarms all night.
11. **Boot-storm attribution ANSWERED from the live second storm** (the
    routed item 33, and the sibling's §f33/§g3): load ~1000 = the agent
    fleet itself (~15 concurrent crush sessions at 16–28% CPU each, 18
    users, plus kith-crm) — CPU saturation with memory PSI near zero, which
    corrects the reboot-risk model: CPU load does not kill the wave; memory
    pressure does. Annotated in place. Commit `b01ce83`.
12. **CI green on every pushed HEAD through `807585c`** on both remotes
    (release law held all session); docs-gates 6/6 PASS verified by direct
    exit code after every docs change.

## b) PARTIALLY DONE

1. **Attempt-5 recovery is STAGED, launch blocked on the live storm**:
   forensics archived, skip count re-verified (228,416), run-id file
   correct, forwarder confirmed dead (restart pending), but memory PSI
   avg10 was 70% at 07:2x (the third storm, in progress) — launching into
   it would repeat the morning's oomd→reboot cycle. A PSI watch loop is
   running (10-min polls, launches only after PSI < 10%); the forwarder
   restart + wrapper relaunch + verification are the next actions the
   moment it clears.
2. **The doom-loop arithmetic is identified but not yet landed as an
   owner ask**: three reboots in ~17 h (09:19, 15:4x, 02:25) vs the gate's
   12.9 h requirement means attempt-2-targeted runs need one clean ~13 h
   window. The multi-run skip union (sibling §e6) reduces ANY resume to
   ~30k scans (~2.3 h) and makes the census reboot-proof; it changes scan
   CLI semantics, so it is an owner decision — the ask is drafted in this
   report (§g1) and lands in the decision package after your go.
3. **Supervision coverage**: beats ran every 20-30 min while the session
   was alive; the 02:21→07:18 interruption (same storm as reboot #3) is
   the uncovered gap — supervision has a single point of failure (my
   session's liveness), see §e3.
4. **CI on the last two pushes** (`b01ce83` and this report's) not yet
   watched to green — queued as the first action after this report.

## c) NOT STARTED

1. **H2 shellcheck on the census scripts**: the freeze technically lifted
   when reboot #3 killed the wrapper (nothing is executing them right
   now), but attempt-5's relaunch is imminent, so the safe sequencing is
   fix-then-launch in one step (new-inode installs: SC2034 in
   census-gate.sh, quoting in census-wave.sh, watchdog + snapshot scripts).
2. **R4 asciinema demo** — needs a free single-flight slot; none exists
   until B exits.
3. **The owner-gated census chain** (02-18 package asks #1-#3): RulesetV2
   stamp before wave C's binary, wave-C launch mode, poisonfix-25 — plus
   GPG (#5), BuildFlow#56 rebuild (#6), Nominet send (#7), M365 surface
   (#8), www-strip rescan slot (#4).
4. **T14-on-FAIL policy** (ask #2 variant): the wrapper arms the cleanup
   loop automatically on a gate FAIL; unattended handling wants your word.
5. **T15 final whole-census distribution** — blocked behind wave C +
   poisonfix closing.
6. **R6 next docs-health pass** — blocked behind the closing report.
7. **zone-ch scan execution** — prep is done (census.md §R3); the scan
   itself waits for a post-C slot.
8. **TCP-contention measurement** (`incoming/outgoing-num-tcp` at w8 →
   NOTES.md) — post-B.
9. **Multi-run skip-union implementation** — owner-gated (§g1).
10. **crt.sh truncation API probe** (identical 2,060-row .de harvest two
    days running) — parked as external-service investigation.
11. **Snapshot keep-N retention policy** — owner call (3 tars ≈ 1.9 GB).
12. **Host-ops repairs** (system_health.prom collector, smartd
    verification, swap posture, PMA MemoryMax budget, GPU GTT) — routed to
    host ops; the attribution answer (fleet CPU saturation) is recorded.

## d) TOTALLY FUCKED UP

1. **I repeated the pipe-ate-my-exit-code gotcha the SAME DAY it was
   recorded.** `docs-gates run . | tail -3 && git commit …` — the `&&`
   chained off `tail`'s exit 0, so commit `d0718d4` landed and PUSHED
   while the freshness gate was red (red from the sibling's brand-new
   report landing, not my edit — but I shipped onto red unknowingly). The
   sibling's 09-58 report §d3 documented this exact failure hours earlier.
   Caught within minutes by re-running with a direct exit code and fixed
   with a follow-up commit — but "recorded twice, done anyway" is now
   "recorded three times". Rule going forward: exit codes only via direct
   runs or `PIPESTATUS`, never a trailing pipe.
2. **My first attempt-4 relaunch omitted `GDD_RESOLVER`**: the wrapper
   defaults its probe to 127.0.0.1:5353, pre-flight failed, exit 5 — one
   wasted launch cycle. The sibling's working launch had it in argv (which
   I had even seen in `ps` output); census.md's ritual prose does not name
   the env. Root cause: executing a documented ritual that omits a
   load-bearing variable. The ledger row now records the full env line.
3. **The overnight supervision blackout**: the storm that caused reboot #3
   (02:25) also killed my heartbeat (session interrupted 02:21 → 07:18).
   Reboot #3 was detected ~5 h late — the exact failure mode the cadence
   exists to prevent, and the same lesson as the sibling's §d1. Structural
   cause: supervision is single-threaded through my session's liveness;
   census-watchdog is machine-verified but dies with the host too, so
   NOTHING survives a host reboot except the journal and me.
4. **Forensics archive was thinner than the runbook I was executing**: I
   archived monitor.log + jsonl line count, not the jsonl itself (§e4 of
   the 09-58 report asks for both). It turned out not to matter (the
   journal is truth and attempt-3's file state is reconstructable), but
   surviving on luck instead of the checklist is how the next one is lost.
5. **I never reconciled whether the wrapper truncates or appends
   run1.jsonl on relaunch** — I observed 73,073 lines surviving one launch
   and a 61 MB file (attempt-4's writes) after the next, and did not pin
   down the mechanism. §e4's runbook entry says "truncates"; my evidence
   is ambiguous. Unverified claim in a runbook = a trap for the next
   recovery.
6. **What I plain forgot**: PSI check before launching attempt-4 (I
   checked memory availability, not memory PRESSURE — the load-1042 storm
   hit 30 min into that wave; it survived on luck and I/O-bound design);
   CI watch on the final two pushes; writing the doom-loop ask into the
   decision package before the overnight (was mid-plan when interrupted).

## e) WHAT WE SHOULD IMPROVE

1. **Check in a reboot-recovery script** (new inode, e.g.
   `scripts/census-resume.sh`): archive forensics → verify/derive skip
   count from the journal → probe/restart the forwarder → relaunch the
   wrapper with the FULL env → verify the three launch lines → append the
   ledger row. Three manual recoveries in two days, one env-var mistake —
   the ritual has earned a program.
2. **PSI guard in the wrapper's pre-flight**: it probes upstreams but not
   memory pressure; a `PSI avg10 ≥ 20% → refuse to launch` check (with the
   recorded morning precedent as rationale) would have refused both my
   launches a few minutes earlier than the storms arrived, and stops
   agents from launching into oomd windows. Also probe it BEFORE
   snapshotting/gating, not just before scanning.
3. **A second clock for supervision**: the cadence dies with my session.
   Cheapest honest fix: a user timer that appends a liveness line to
   monitor.log every 30 min and alarms (gatus already watches the host)
   when the wave is up but no line lands — making MY blackout machine-
   detected instead of luck-detected.
4. **The multi-run skip union** (sibling §e6, now urgent): one run id per
   resume makes every reboot cost a full 12.9 h gate-run; a union of two
   ids (or wave-scoped skip) collapses any resume to the ~30k never-scanned
   tail (~2.3 h) and is reboot-proof. It is a scan CLI semantic change →
   owner ask #9 (§g1).
5. **Exit-code hygiene as a personal lint**: no `| tail` on any command
   whose exit code gates an action; use direct runs or `PIPESTATUS`.
   Three recordings and one real incident — it needs to be a reflex, not a
   note.
6. **Runbook claims need one verification**: before the next recovery,
   determine empirically whether census-wave.sh truncates or appends
   run1.jsonl (one `strace`-free test on a throwaway dir) and correct §e4
   accordingly.
7. **Archive the FULL jsonl** (or at least its sha256 + head/tail) in the
   recovery script — metadata-only archives assume the file survives, and
   §d4 shows what that assumption is worth.
8. **Fleet memory budget is the actual root cause**: three oomd-reboots in
   17 h with the same signature. Every census hour is hostage to it. This
   is host ops + the owner, but the census now has hard numbers on what it
   costs (§c12/§g2).
9. **Milestone beats over silent beats**: my supervision logs were lean
   (good for noise), but writing the doom-loop arithmetic at 16:00 instead
   of ~20:00 would have given the owner a business-hours decision window.

## f) UP TO 50 THINGS TO GET DONE NEXT

**Census critical path (in order):**

1. Land this report + index row + push; watch CI green on it (release
   law).
2. Wait out the live storm (PSI watch loop is running); the moment
   PSI avg10 < 10% twice: restart unbound-tcp, relaunch the wrapper with
   `GDD_RESOLVER=127.0.0.1:5354 GDD_PREFLIGHT_TRANSPORT=tcp
   GDD_UNBOUND_CONF=scripts/unbound-tcp.conf`, verify the three launch
   lines + expect-skip 228,416, re-attach the watchdog.
3. Record attempt-5's launch in the census.md ledger row (attempt 4's
   death at 140,981 + reboot #3 + the staged recovery).
4. Heartbeat cadence with PSI in every beat (memory, not just jsonl).
5. Owner answer §g1 (multi-run skip union) → if GO: implement union
   resume (scanstore LoadResumeSource + scan flag, ~2 h) — every future
   recovery becomes ~30k scans.
6. On B exit: verify wrapper auto-gate PASS
   (`--manifest --expect-skip 228416` — note the NEW arithmetic: scanned
   171,584 + skip 228,416 = 400,000).
7. If gate FAIL (exit 4): T14 cleanup loop per your §g3 answer (the
   wrapper's exit-4 path snapshots and stops; cleanup is a separate
   decision).
8. Confirm auto-snapshot flowed on B's exit; restore-verify B's OWN
   snapshot (the last 1/5 remainder of the post-B bundle).
9. Extract attempt-4's run id into the ledger (audit provenance, same as
   §f3 of the 09-58 report did for attempt 2).
10. census.db headroom re-check for wave C + zone-ch.
11. Owner answer (RulesetV2): stamp `v2-2026-11-14` BEFORE wave C's
    binary exists (10-min plan in the decision note).
12. Owner answer (launch mode): wave C attended vs unattended.
13. Build wave C's binary + PROVENANCE stamp + sha256.
14. Pre-flight + launch wave C (`tranco1m-c`, 500,001–1,000,000, w4/10s,
    5354, TCP) + fresh watchdog.
15. poisonfix-25 (fresh binary, 25-domain list) on owner go.
16. Closure proof: `replay --audit-excluded <4 run IDs>` → 0 poisoned;
    append the ledger row.
17. T15 final whole-census distribution in REPORT.md (journal-derived,
    replay command pinned, Microsoft-evidence runs labeled per the
    ruleset question).
18. Closing status report + index row + CHANGELOG census line + push +
    CI green (the closeout ritual).
19. H2: shellcheck the census scripts (SC2034 + quoting) — fix-then-launch
    in ONE step on new inodes, before the next wrapper relaunch if
    possible.
20. Re-run shellcheck across scripts/ and record zero-or-rationale.
21. Empirically pin run1.jsonl truncate-vs-append and correct the 09-58
    §e4 runbook entry.
22. Write the census-resume.sh recovery script (§e1) and use it for any
    reboot #4.
23. Add the PSI pre-flight guard to census-wave.sh (§e2) — same new-inode
    step as 19.

**Repo hygiene / gates:**

24. CI watch on this report's push + keep the release law through the
    chain.
25. TODO_LIST strike sweep after wave B closes (the wave rows + this
    session's staged rows).
26. FEATURES.md PARTIALLY rows re-check after wave C (vocabulary already
    normalized).
27. `buildflow -s go-auto-upgrade` once post-census (lo-hint set drift
    check).
28. scanstore_test.go 328/350 — pre-split before any addition >22 lines.
29. docs-gates freshness after every status report (this one included).
30. CHANGELOG current through the wave-C window.

**Cross-repo / upstream:**

31. File the projectionhost settled-API issue on owner go (draft ready).
32. Hand the CRM delta-retirement confirm to the crm session/owner (this
    side is done and recorded).
33. go-cqrs-lite patch sweep re-check post-census (pins verified latest
    today).

**Post-census / parked:**

34. zone-ch scan execution (6 × 450k batches, separate zone-ch.db,
    pilot-gated — plan ready in census.md §R3).
35. asciinema demo recording (needs a scan slot).
36. TCP-contention measurement → unbound NOTES.md.
37. crt.sh truncation API probe before trusting the 04:37 feed for
    zone-ch planning.
38. Verdict-flip daily digest timer.
39. Lead scoring v1 weights from the final census distribution.
40. Enrichment join / registrant dedup framing polish (ROADMAP bullets
    exist).
41. Journal compaction decision when census.db passes a few GiB.
42. Whole-.uk pipeline when Nominet credentials arrive.
43. R6 next docs-health pass (archive the 11-14 report generation,
    TODO hygiene re-run).
44. Snapshot keep-N retention decision (owner).
45. Poller config comments: record the per-band unknown-rate + none-share
    bounds next to GDD_WARN_PCT (numbers live in TODO_LIST evidence).
46. AGENTS.md diet candidate list (219/220 — the census gotcha block
    could point to census.md §incident instead of duplicating).
47. Host-ops: system_health.prom collector, smartd verification, swap
    posture, PMA MemoryMax, GPU GTT (routed; fleet attribution recorded).
48. `--resume latest` hard-warning/alias-removal polish (it bit wave A;
    the wrapper works around it).
49. Status-report pipeline: keep index rows + gates green per report
    (ritual, not a task — listed for the closeout).
50. Whole-census closure proof bundle: final REPORT.md + closing report +
    tags decision (go-release law: CI green on the exact pushed commit
    before any tag).

## g) THREE QUESTIONS I CANNOT FIGURE OUT MYSELF

1. **May I implement the multi-run skip union** (ask #9, extends the
   02-18 package)? Three reboots in 17 h vs a 12.9 h gate requirement is a
   structural doom loop; a union-of-resume-ids (or wave-scoped skip) turns
   every recovery into ~30k scans (~2.3 h) and makes the census
   reboot-proof. It changes scan CLI semantics (scanstore
   `LoadResumeSource` + the scan flag + the wrapper's expect-skip
   arithmetic), which is why it has been sitting in owner-territory since
   the sibling flagged it (§e6). ~2 h of work, pinned by tests, zero
   journal-format change.
2. **Pause or push through?** With the host rebooting every ~6 h under
   fleet memory storms, do you want the census PAUSED until host-side
   stabilization (PMA budget, GTT, oomd tuning — the attribution answer
   says CPU storms ride fine, MEMORY storms kill), or keep relaunching
   through reboots (journal-safe; the gate needs one clean ~13 h window
   either way)? I will keep the PSI-gated relaunch discipline either way.
3. **Unattended chain pre-authorization** (carries asks #1-#3): when B's
   gate eventually PASSES, do I run B→gate→(T14 if FAIL)→RulesetV2
   stamp→wave C→poisonfix→closure unattended, or hold at each stage? B's
   exit is now plausibly Friday evening or Saturday; every handoff you
   pre-authorize removes one dead-hours stall. The 02-18 package's 8 asks
   still stand; this is the same decision under the new reboot cadence.

---

**Pushed this session:** `2ab65df`, `3a76e78`, `476a52a`, `d0718d4`,
`807585c`, `b01ce83` (both remotes; CI green through `807585c` — the last
two ride CI watch as §b4). **Live state at writing:** no scan (reboot #3
at 02:25 killed attempt 4 at 140,981 scanned; journal holds everything);
attempt-5 staged; PSI watch loop running; tree clean; origin = `b01ce83`.
