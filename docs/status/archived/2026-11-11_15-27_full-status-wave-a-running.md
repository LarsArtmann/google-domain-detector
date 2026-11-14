> **Live remainder:** every item resolved as of the 2026-11-13 docs-health pass (docs-gates green); open census work lives in TODO_LIST.md + docs/census.md.

# Session status — FULL: E10 closed, wave A at 21%, all session debt routed; waiting

**2026-11-11 15:27 CET · @ master 0316f84 (+1 uncommitted report reword;
both pushed after this) · CI GREEN on tip (1m58s) · census lane · wave A
pass 1: 17 270/90 834 scanned (19.0%), ~1.55 dom/s, 1.75% unknown —
steady UNDER the 2% D3 bar since launch; ETA ~04:00–06:00 tonight**

Second full a–g at owner request (first: 14-44, forty minutes ago;
this one supersedes it). Same session arc: resumed the 19-04 handoff,
verified the stale parts, closed E10, measured the sustained-load
question into a wave-parameter decision, launched wave A, fixed the CI
red, and routed every piece of session debt. Nothing new was started
since 14-44 except debt-routing; wave A ran from 13.6k to 17.3k
domains.

## a) FULLY DONE (this session, cumulative)

1. **Handoff reconciliation** — every "pending" 19-04 item verified
   against git/GH before touching: fuzz test + report daemon-swept
   intact (03e0682), sibling's red tree already fixed, full `-race`
   green on the reconciled tree, all pushed, CI green.
2. **E10 legacy-unknowns recovery CLOSED** — round 3 (113 domains,
   w2/12s warm): 67 resolved, 5 dead, 28 servfail, **13 residual =
   0.18% < 0.5% bar**; STOP rationale recorded. Final: 4 732 resolved /
   1 864 dead / 595 broken delegations / 13 residual
   (reports/census/README.md + NOTES.md + three journal runs). T09
   struck (pre-flight SAFE, annotate PASS).
3. **D3 servfail-stable refinement** — stable-SERVFAIL = explained
   broken delegation, never silent: docs/census.md decision +
   `census-gate.sh --servfail-stable` opt-in flag, self-tested both
   modes, plain output byte-compatible.
4. **E16 cleanup loop** (`scripts/census-cleanup.sh`) — falling workers
   4→2, bar on ORIGINAL wave size (a round's own rate is meaningless on
   100% prior errorers). Stub-tested: convergence, over-bar FAIL +
   levers, usage/path errors.
5. **CI red FIXED** (b22b6fc) — env-dependent coverage test asserting
   `defaultRangesCache()` mkdirs; reproduced with clean HOME (red),
   assertion removed (green), vet+suite+gofmt clean. **CI green
   confirmed on tip once GH connectivity returned** (~12:33 CET); the
   outage window covered four intermediate pushes.
6. **audit-freshness guard** (E31.4, c6dca0c) — 22 audit pins vs
   go.mod, exit 1 on drift; fresh + stale paths verified; 2 pinned
   fresh, 20 prospective SKIPs today.
7. **Sustained-load soak ladder** — w8 Quad9-only 26.0% · w8 +Cloudflare
   21.1% · w4 +Cloudflare 5.35%* (upper bound, pilot-contaminated). The
   E9 ladder's 1.5%-at-w8 was a 50-second window; real w8 qps is
   ~140–170, not 10–30 (unbound.conf comment corrected).
8. **Wave parameters decided + recorded** (3b3ae41): w4/10s/Quad9+
   Cloudflare, two-pass protocol, D3 gate post-cleanup. Wave A ≈16–18 h,
   1M ≈7–8 days.
9. **Single-flight law + pilot exclusion** — one census scan at a time
   (collision proven); sibling's 4 999-domain pilot that ran on the
   SYSTEM resolver (127.0.0.1:53) excluded from census accounting.
10. **Wave runner** (`scripts/census-wave.sh`, 15c1203) — pass → gate →
    cleanup → verdict, monitor.log, setsid-detached, stub-tested
    end-to-end (after fixing my own zombie-race poller design).
11. **WAVE A RUNNING since 12:25 CET** (pid 92166) — resumes past soaks
    1–3, journal wave `tranco1m-a`, self-managing. Live: 1.75% unknown,
    on pace to pass the gate on pass 1 without cleanup.
12. **Session documentation loop closed** — 12-35 handoff report, 14-44
    self-review, AGENTS gotchas (jq shim + null-filter semantics;
    census single-flight), retract-note row routed, wave-sequencing and
    crt.sh-scheduling rows routed, tokenless report line reworded to
    bind (T13). Freshness gate: PASS on the newest report.

## b) PARTIALLY DONE

- **Wave A pass 1 in flight** (~19%, ETA early morning). Completion,
  T11-A strike, and wave B launch need an awake session (~04:00+).
- **docs-gates audit-all red** — 27 unharvested §f items in SIBLING
  reports (sales 19-27: 18, E24 02-00: 4, E24 19-08: 4, 13-28: 1).
  Their lanes were ACTIVE; flagged in both reports, not harvested.
- **docs-gates cadence red** — 16 unarchived status reports (threshold
  15). The archive ritual is a bulk `git mv` over reports including
  sibling-lane ones; deliberately awaiting owner/lane go-ahead, not a
  unilateral move.
- **Outage-window CI runs** — tip green covers the lineage; the four
  individual runs (b22b6fc→92432b0) were never individually eyeballed.

## c) NOT STARTED (deliberate)

- Waves B and C (single-flight sequential after A).
- E17 census report (needs waves; must pin the excluded pilot run ID
  01M74VNNCJS6… machine-greppably).
- CLI-lane tail (sibling ACTIVE in cli/scanstore): interrupted-run
  stderr summary wart, runs-table metadata race, describe contract
  test (E31.2), TimeoutMiddleware (E31), E29 CSV enrichment, varnamelen
  policy.
- Retract-note dry run (routed). IPv6 --resolver check (needs v6
  resolver). 7s-vs-10s timeout experiment. w2 sustained soak (fallback
  data — only if wave A drifts).

## d) TOTALLY FUCKED UP (owned; full detail in 14-44 §d)

1. **Unwatched push → master red ~4 h** (E16 lineage carried a
   sibling's env-dependent test via daemon mixing). Habit existed;
   violated. Every push now gets a run-watch or an explicit deferral.
2. **Clock fiction in NOTES** — wrote assumed times (02:0x) where
   artifacts said 08:2x; corrected from file evidence.
3. **Nearly committed a false gotcha** — blamed the jq shim for what
   was plain null-filtering; caught on honesty re-check pre-commit.
4. **Single-flight law lived in my head until the collision** — never
   `pgrep`'d before soaks 2–3; the concurrent pilot contaminated soak 3
   (decision made on noisier data than necessary).
5. **Zombie-race poller in the first census-wave.sh** — caught by my
   stub test; rewritten foreground-scan + background-poller.
6. **census-slice.sh SIGPIPE bug shipped by me, fixed by a sibling**
   (tr-then-head under pipefail).
7. **Two buggy test harnesses** (stray placeholder line; stale-path
   test missing its fixture tree) — each burned a cycle before the
   real test ran.
8. NEW since 14-44: **two routing rounds needed for one tokenless
   report item** ("Wave C after B." — every token generic, first
   routing attempt couldn't bind). Reworded the report line to carry
   "T13". Lesson: write §f items with at least one distinctive token
   (an ID, a number, a proper noun) so the freshness gate can bind
   them at all.

## e) WHAT WE SHOULD IMPROVE (systemic)

1. **Census journal has NO backup** — census.db (13 MB+, sole copy,
   ~3 GiB projected) on one gitignored disk. Snapshot routine needed;
   destination is an owner call (§g).
2. **Running binary provenance unstamped** — wave-A binary built from
   the 20:15 Nov-10 tree; decision to keep it was right, the sha256 +
   source-commit record was skipped.
3. **No mid-pass degradation alert** — wrapper logs hourly progress but
   never thresholds it; a throttle window would only surface
   post-pass.
4. **audit-freshness.sh runs nothing** — exists, tested, unwired
   (CI/docs-gates candidate).
5. **Excluded pilot run not pinned by ID** — prose-only in census.md;
   E17 needs the run ID greppable.
6. **§f-item authoring discipline** — items need distinctive tokens to
   be harvestable (today's two-round routing).
7. **w2 sustained + 7s-timeout data absent** — fallback levers have no
   measurements behind them.
8. **Archive cadence now hard-red** — the ritual needs scheduling
   (owner/lane call); it will keep tripping every new report until
   done.

## f) NEXT (up to 50, ranked; ⏳ waits on wave A, 🔀 sibling lane)

1. ⏳ Watch wave A pass-1 completion; verify wrapper gate/cleanup from
   monitor.log.
2. ⏳ Strike T11 wave-A half on green verdict.
3. ⏳ Launch wave B (single-flight, same wrapper, fresh wave id +
   manifest slice).
4. ⏳ Wave C (the T13 slice, 500k–1M) after wave B completes.
5. ⏳ E17 census report from the journal (pin excluded pilot run ID;
   document soaks methodology + selection-bias warnings).
6. Strike T14 per wave as cleanups land (may be zero rounds for A if
   1.75% holds).
~~7. Wire audit-freshness.sh into CI or docs-gates.~~ done — CI lib-freshness job (scripts/lib-freshness.sh)
8. Census journal snapshot routine (destination per §g3).
9. Stamp running binary sha256 + source commit into wave dir/NOTES.
10. Mid-pass unknown-rate WARNING in census-wave.sh monitor loop.
11. Verify tonight's 03:17 UTC nightly-soak cron (first scheduled fire
    was green 14m39s; tonight is the second).
~~12. Retroactively eyeball the four outage-window CI runs.~~ done — outage-window runs reconciled (00-03 patch + nxfix supersession; ~3,450 garbage observations explained in AGENTS.md)
13. 7 s vs 10 s per-domain timeout experiment (30-min w4 soak).
14. w2 sustained soak only if wave A drifts above bar.
15. 🔀 Interrupted-run stderr summary fix (cli; TODO row filed).
16. 🔀 Runs-table metadata shutdown race (cli/scanstore; soak 3 row
    stuck "started, 0 domains", all 2 731 observations replay fine).
17. 🔀 describe contract test (E31.2).
~~18. 🔀 TimeoutMiddleware whole-run budget (E31).~~ done — `--max-runtime` (E31)
19. 🔀 E29 CSV enrichment.
20. 🔀 varnamelen `f` policy + final buildflow format pass.
21. Harvest/route sales-lane 19-27 §f (18 items) — theirs or by policy
    (§g2). <!-- fresh:skip sibling-scoped pending owner policy -->
22. Harvest E24 02-00/19-08 §f (8 items) — same.
    <!-- fresh:skip sibling-scoped pending owner policy -->
23. Archive ritual for 16 status reports (cadence red; bulk git mv +
    check-rows first).
24. Retract-note dry run (routed; go-release lane).
25. IPv6 --resolver handling check (needs v6-capable resolver).
~~26. TXT truncation investigation (existing TODO row).~~ **Superseded — closed with live evidence 2026-11-13 (00-03 §8) — resolver distress, not truncation.**
27. E28 --pre-filter (plan v3; .ch AXFR feeds it).
28. .ch SWITCH AXFR zone-pull tooling (existing TODO row).
~~29. crt.sh .de rate-limit-aware scheduling (routed today).~~ done — crtsh-daily.sh + systemd timer
30. reports/ durability policy decision (E1 README + census db class;
    owner policy, one decision applied everywhere).
31. E17 export: compact verdict table + `replay --since` demo for the
    sales narrative.
32. README: document `replay --since` (the delta/sales signal exists,
    under-marketed).
~~33. E33 coverage 69→90 finish (sibling mid-flight).~~ done — 90.7% (2026-11-13)
34. E25 lint lane final zero-gate (sibling).
35. Nightly soak: add a wave-A heartbeat/alert job (a stalled wave
    currently pages nobody).
36. Scanner-side `--qps` pacing knob — design only, after census data.
37. STACK.md row for the census scripts (census-wave/cleanup/gate).
38. CHANGELOG [Unreleased] entries for census tooling.
39. README census section once first population numbers exist.
40. Session-habit: single `git mv` archive of THIS session's reports
    when the ritual runs, so the next session starts under threshold.

## g) QUESTIONS (cannot answer myself)

1. **Census throughput spend:** accept ~7–8 days for the 1M at w4
   (current, healthy), or fund an offsite resolver/VPS to try unblocking
   higher worker counts for waves B/C? _(Default: accept w4.)_
2. **Cross-lane harvesting:** when sibling lanes go quiet with unharvested
   §f items (audit-all is red on 27 foreign items), may I harvest them
   into TODO_LIST, or is strict lane ownership worth the red gate?
   _(Default: leave foreign lanes alone, keep flagging.)_
3. **Census-journal backup destination:** census.db is the census's
   single copy (gitignored by design). Snapshots to another LAN machine
   (evo-x2?), external drive, or local-only tarballs next to the db?
   I can build the routine; the destination is infrastructure I can't
   pick. _(Default: local tarball per completed wave.)_

---

**Live state:** wave A pass 1 at 17 270/90 834 (19.0%), 1.75% unknown,
unbound healthy, wrapper detached/self-managing, CI green, tree clean
except this report. **WAITING FOR INSTRUCTIONS.**
