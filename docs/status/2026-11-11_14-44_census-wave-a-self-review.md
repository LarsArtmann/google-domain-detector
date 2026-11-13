# Session status — E10 closed, soaks decided wave params, wave A RUNNING; full self-review

**2026-11-11 14:44 CET · @ master 92432b0 (pushed both remotes; CI GREEN
on tip — connectivity returned ~12:33 CET, the b22b6fc fix verified) ·
census lane · wave A pass 1: 13 565/90 834 scanned, 1.63 dom/s, 1.77%
unknown (under the 2% D3 bar), ETA ~04:00 tonight**

Resumed the 19-04 handoff ("GET SHIT DONE — the WHOLE TODO LIST"),
executed the census lane end-to-end, and am now in monitor mode on the
first real wave. This report covers this session only.

## a) FULLY DONE (this session)

1. **Handoff reconciliation before redoing anything.** All "pending"
   handoff items verified against reality: fuzz_test.go + the 19-04
   report were daemon-swept intact (03e0682); the sibling's red tree
   was already fixed; full `-race` suite re-run green on the
   reconciled tree; all of it pushed, CI green.
2. **E10 legacy-unknowns recovery CLOSED.** Round 3 (113 timeout-class
   domains, w2/12s warm): 67 resolved, 5 dead, 28 servfail, **13
   residual = 0.18% of 7 204, under the 0.5% cleanup bar — STOP
   decision documented (diminishing returns on a monotonically
   shrinking pathological set)**. Final accounting (4 732 resolved /
   1 864 dead / 595 broken delegations / 13 residual) in
   `reports/census/README.md` + NOTES.md; census.db holds all three
   journal runs; T09 struck (docs-gates pre-flight SAFE, annotate PASS).
3. **D3 servfail-stable refinement.** Stable-SERVFAIL = domain-side
   broken delegation = _explained_, never silently: documented in
   docs/census.md, encoded as explicit opt-in
   `census-gate.sh --servfail-stable`, self-tested both input modes,
   plain output byte-compatible with the archived expectations.
4. **E16 cleanup loop** (`scripts/census-cleanup.sh`): falling workers
   (4→2), bar on the ORIGINAL wave size (a round's own rate is
   meaningless — input is 100% prior errorers). Stub-tested:
   convergence, over-bar FAIL with next levers, usage/path errors.
5. **CI red FIXED (b22b6fc).** A daemon-mixed commit carried a
   coverage test asserting `defaultRangesCache()` leaves its dir on
   disk — true only on machines that ran a scan before. Reproduced
   with a clean HOME, removed the wrong assertion (the getter is pure;
   the loader mkdirs lazily). CI green confirmed on tip after
   connectivity returned.
6. **audit-freshness guard (E31.4, c6dca0c).** Parses the 22 audit
   pins, compares against go.mod, exit 1 on drift. Fresh + stale paths
   verified (the stale path initially tested wrong — temp dir lacked
   docs/; retested with symlinked structure). Currently: 2 pinned
   audits fresh, 20 prospective SKIPs.
7. **Sustained-load soak ladder (§g1 answered by measurement).**
   Three 30-min soaks through the census forwarder: w8 Quad9-only
   **26.0%** unknown · w8 Quad9+Cloudflare **21.1%** · w4 + Cloudflare
   **5.35%** (upper bound — contaminated by a concurrent sibling
   pilot). The E9 ladder's "1.5% at w8" was a 50-second window that
   does not hold; real census qps at w8 is ~140–170, not the assumed
   10–30 (unbound.conf design comment corrected).
8. **Wave parameters decided + recorded (3b3ae41):** w4 / 10s /
   Quad9+Cloudflare, two-pass protocol (pass 1 leaves ≤5%, E16 cleanup
   drives <0.5%, D3 gate POST-cleanup). Wave A ≈16 h, full 1M ≈7–8
   days. Ladder table + rationale in docs/census.md.
9. **Single-flight law + contamination exclusion.** ONE census scan at
   a time (proven collision); the sibling's 4 999-domain pilot that ran
   on the SYSTEM resolver (127.0.0.1:53 — protocol violation,
   concurrently with my soak 3) is excluded from census accounting.
10. **`scripts/census-wave.sh` wave runner (15c1203):** pass → gate →
    cleanup → verdict, all to monitor.log, launched detached (setsid)
    so it outlives the session. Stub-tested end-to-end.
11. **WAVE A LAUNCHED 12:25 CET** (pid 92166), resuming past soaks 1–3
    (zero double-scan; resume semantics proven against SIGINT-
    interrupted runs). Currently 1.77% unknown — the two-pass design
    may not even need the cleanup loop.
12. **Session docs:** status report 12-35 (handoff-grade), AGENTS
    gotchas (jq shim + null-filtering semantics, census single-flight),
    retract-note TODO row routed (docs-gates freshness now PASS on the
    newest report).

## b) PARTIALLY DONE

- **Wave A pass 1 in flight** (~13 h left). The wrapper self-manages
  (gate + cleanup + verdict in monitor.log); nothing to babysit, but
  completion + T11-A strike + wave B launch need a session awake at
  ~04:00.
- **CI verification:** tip green; the four intermediate pushes
  (b22b6fc→92432b0) were pushed during the GH-API outage — tip green
  covers the fix, but nobody eyeballed each intermediate run.
- **docs-gates audit-all still red** — 28 unharvested §f items across
  four reports; 27 of them belong to sibling lanes (sales 19-27: 18,
  E24 02-00: 4, E24 19-08: 4); mine (19-04: retract-note) is now
  routed. Flagged, not harvested — their lanes were ACTIVE.

## c) NOT STARTED (this session, deliberately)

- Waves B (100k–500k) and C (500k–1M) — strictly sequential after A
  (single-flight law).
- E17 journal-derived census report (needs completed waves; must encode
  the pilot-run exclusion by run ID).
- CLI-lane tail — sibling actively in cli/scanstore: describe contract
  test (E31.2), TimeoutMiddleware (E31), E29 CSV enrichment, the
  interrupted-run stderr-summary wart (TODO row filed this session).
- Retract-note dry run (row routed this session).

## d) TOTALLY FUCKED UP (owned, with lessons)

1. **Pushed E16 and did not watch CI — master sat RED ~4 h.** The
   break was a sibling's env-dependent test that the daemon mixed into
   the lineage, but MY push carried it and the AGENTS watch-habit
   exists exactly for this. The fix took 5 minutes once seen.
   _Lesson: every push gets a run-watch before the next work block; if
   GH is unreachable, the deferred-verification debt goes in the
   session report IMMEDIATELY, not in a later section._
2. **Wrote wall-clock times from an assumed internal clock.** NOTES
   said "02:05–02:35"; the artifacts said 08:22–08:52. Caught while
   writing the 12-35 report, corrected from file evidence.
   _Lesson: never write a time into a record without reading it off
   the artifact._
3. **Nearly committed a false gotcha.** First AGENTS draft blamed the
   jq shim for "silently emitting 1 373 of 3 153 rows" — that was
   plain jq null-filtering (`select(.error_kind)` drops clean rows),
   NOT a shim bug. Caught on an honesty re-check before commit.
   _Lesson: verify the MECHANISM of a claim before encoding it into
   memory; "it surprised me" is not a diagnosis._
4. **Single-flight discipline existed only in my head until the
   collision.** I never ran `pgrep` before soak 2/3; the sibling's
   concurrent pilot contaminated soak 3's measurement (5.35% is an
   upper bound, decision made on noisier data than needed).
   _Lesson: coordination rules go into the shared runbook BEFORE the
   runs that need them._
5. **First census-wave.sh design hung on a zombie race** (kill -0 on
   an unreaped dead child → one blind 30-min sleep). Caught by my own
   stub test, rewritten to foreground-scan + background-poller.
   _Lesson: poller patterns around child processes need reaping
   semantics thought through up front._
6. **census-slice.sh shipped with a pipefail SIGPIPE bug** (tr-then-
   head: head's early exit kills tr, exit 141 under pipefail). MY
   script; a sibling found and fixed it. _Lesson: every script I
   write under `set -euo pipefail` gets a pipefail-path test, not just
   the happy path._
7. **Two stub-test harnesses were themselves buggy** (a stray
   placeholder line aborted run 1; the stale-path test lacked
   docs/review/libs and silently exercised the wrong exit). Both
   caught by reading outputs — but each burned a cycle.
   _Lesson: test the test before trusting its red/green._

## e) WHAT WE SHOULD IMPROVE (systemic, from this session)

1. **The census journal has NO backup story.** census.db (13 MB+, the
   only copy, growing to ~3 GiB at 2.5M scale) lives on one disk under
   gitignored reports/. A disk failure mid-census loses everything.
   Needs at least periodic snapshot/rsync; ideally the wrapper's
   monitor loop snapshots after each pass.
2. **Running-binary provenance is unstamped.** The wave-A binary was
   built 2026-11-10 20:15 from a tree that has since changed
   (behavior-preserving refactors + one test fix). Decision to keep it
   (continuity) was right; recording WHICH commit it was built from
   was skipped. Stamp sha256 + source commit into NOTES/wave dir.
3. **No mid-pass degradation alert.** The wave wrapper logs hourly
   progress but never evaluates the unknown-rate against a threshold
   — a throttle window would only be seen in the post-pass gate. Add a
   WARNING line to the monitor when a progress window exceeds ~2× the
   expected rate (log-only; killing passes is an operator call).
4. **audit-freshness.sh is not wired into anything** — it exists, is
   tested, and nothing runs it. Belongs in CI (fast, offline) or the
   docs-gates battery.
5. **Pilot-run exclusion is documented but not pinned by ID.** The
   contaminated pilot run (01M74VNNCJS6…) must be excluded in E17;
   census.md says so in prose but the ID should live in a
   machine-greppable place (wave ledger or the census README).
6. **Intermediate CI runs unverified during the GH outage** — a
   "deferred verifications" queue in the session report would have
   made the debt visible instead of remembering it.
7. **w2 sustained was never measured** — if w4 drifts above bar
   mid-wave, the fallback has no data behind it.
8. **Per-domain timeout is unvalidated against throughput.** Timeouts
   burn the full 10 s budget; a 7 s budget might trade marginal
   domains for meaningful throughput — cheap experiment, never run.

## f) NEXT (up to 50, ranked; ⏳ = blocked/waiting, 🔀 = sibling lane)

1. Monitor wave A pass 1 to completion (~04:00); verify the wrapper's
   gate (+ cleanup if triggered) from monitor.log.
2. Strike T11's wave-A half when the pass-1 verdict lands.
3. Launch wave B via `scripts/census-wave.sh` (single-flight: only
   after A fully done incl. cleanup).
4. Wave C (the T13 slice, 500k–1M) after wave B completes.
5. E17: journal-derived census report (include pilot-run exclusion by
   ID, soaks' methodology, selection-bias warnings).
6. Strike T14 per wave as cleanup loops complete.
7. Wire audit-freshness.sh into CI (fast offline step) or docs-gates.
8. Census journal snapshot routine (rsync target or tarball per pass)
   — needs owner input on destination (see §g).
9. Stamp the running binary's sha256 + source commit into the wave
   dir + NOTES.
10. Add mid-pass unknown-rate WARNING to census-wave.sh monitor loop.
11. Pin the excluded pilot run ID in census.md wave ledger.
12. Verify tonight's 03:17 UTC nightly-soak cron fired green (first
    scheduled fire; earlier schedule run at 03:21Z yesterday was
    green, 14m39s).
13. Verify the four outage-window CI runs (b22b6fc→92432b0) went
    green retroactively.
14. 7 s vs 10 s per-domain timeout throughput experiment (one 30-min
    w4 soak).
15. w2 sustained soak (fallback data) — only if wave A drifts.
16. 🔀 Interrupted-run stderr summary fix (cli lane; TODO row filed —
    SIGINT'd runs print N-domains lines counting ~97k synthetic
    canceled fills).
17. 🔀 Runs-table metadata race (cli/scanstore lane; soak 3's row
    stuck "started, 0 domains" while all 2 731 observations replay
    fine).
18. 🔀 describe contract test (E31.2).
19. 🔀 TimeoutMiddleware whole-run budget (E31).
20. 🔀 E29 CSV enrichment.
21. 🔀 varnamelen `f` policy decision + final buildflow format pass
    (their own §b).
22. Harvest/route the sales-lane 19-27 report's 18 unharvested §f
    items (or their session does; docs-gates audit-all red on it).
~~23. Harvest the E24 reports' 8 unharvested items (same class).~~ docs-health pass 2026-11-13: the 19-08 report's lint items annotated this sweep (done at 399385a / superseded / won't)
24. Retract-note dry run (row routed; go-release lane).
25. IPv6 `--resolver` address handling check (E31 row; needs a v6
    capable resolver to test against).
26. TXT truncation investigation (cloudflare "cannot unmarshal" class;
    TODO row exists).
27. E28 `--pre-filter` (plan v3; deferred earlier).
28. `.ch` SWITCH AXFR zone pull tooling (census zone-inputs).
29. crt.sh `.de` extractor rate-limit-aware scheduling (once/day/TLD).
30. Durability policy for gitignored reports/ generally (E1 README
    referenced by release notes suffers the same gap — sibling
    flagged it; decide once, apply everywhere).
31. Census E17 export: compact verdict table + delta-query demo for
    the sales narrative.
32. `replay --since` documentation example in README (the sales
    signal exists but is under-marketed).
33. Coverage push E33 69→90% (sibling mid-flight; finish the half-open
    gap).
34. E25 lint lane final gate (buildflow findings at zero, per their
    02-00 report).
35. Nightly soak workflow: consider adding a wave-A progress heartbeat
    job so a stalled wave pages someone.
36. Consider a scanner-side pacing knob (`--qps`) as the structural
    fix if forwarders keep throttling — design only after census data.
37. Archive cadence: 15 unarchived status reports at threshold —
    start archiving the oldest batch (docs-gates cadence will trip at
    16).
38. STACK.md refresh after the census tooling landed (census-wave.sh,
    census-cleanup.sh are repo-level scripts worth a row).
39. CHANGELOG entries for the census tooling (unreleased section).
40. README census section link to docs/census.md once waves produce
    first population numbers.

## g) QUESTIONS (cannot figure out myself)

1. **Census duration vs. throughput spend:** w4 puts the full 1M at
   ~7–8 days. Accept that, or fund an offsite resolver/VPS (even a
   small one on a datacenter path) to unblock higher worker counts?
   Wave A is already running at w4 and healthy — this decides waves
   B/C, not the current run. _(Default if no answer: accept w4.)_
2. **Cross-lane harvesting policy:** may I harvest/route OTHER
   sessions' status-report §f items into TODO_LIST when their lanes go
   quiet (docs-gates audit-all is red on 27 foreign items), or is
   strict lane ownership worth the red gate? _(Default: leave foreign
   lanes alone, keep flagging.)_
3. **Census-journal backup destination:** census.db is the census's
   single copy (gitignored by design). Where should periodic snapshots
   go — another machine on the LAN (evo-x2?), an external drive, or
   nowhere (accept the risk)? I can build the snapshot routine; I
   cannot pick the destination. _(Default: tarball snapshots next to
   the db per completed wave, local-only.)_

---

**Current live state:** wave A pass 1 at 13 565/90 834 (15.0%),
1.63 dom/s, 1.77% unknown, unbound healthy, wrapper detached and
self-managing. Session is idle-monitoring; **WAITING FOR
INSTRUCTIONS.**
