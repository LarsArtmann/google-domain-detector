> **Live remainder:** every item resolved as of the 2026-11-13 docs-health pass (docs-gates green); open census work lives in TODO_LIST.md + docs/census.md.

# Status 2026-11-12 04:15 — self-review: census hardening session + wave A at 93%

Scope: THIS session (2026-11-11 ~16:00 → 2026-11-12 04:15 CET), census
lane, per owner request. Live state embedded at the bottom.

## a) FULLY DONE (verified this session)

1. **Mid-pass unknown-rate WARNING** in census-wave.sh poller
   (5cf2231): `GDD_WARN_PCT` env (default 2 = D3 bar, 0 disables);
   boundary-tested (3% warns; 2.00% at bar=2 does not; canceled counts
   as unknown; missing file no-crash). Applied via **inode-safe swap**
   (`install`) so the RUNNING wave-A wrapper kept its old bytes —
   verified by inode change 7434877→7549013 + both wrapper pids alive.
   Documented in docs/census.md.
2. **Census binary provenance** stamped (§e2 of 15-27): embedded VCS
   stamp via `go version -m` = `e1f5160` + vcs.modified=true,
   vcs.time 2026-11-10T18:12:02Z, go1.27.2/jsonv2; sha256 recorded;
   byte-reproduction attempted and honestly disproven (build-dir paths
   embedded in Go binaries); decision recorded: waves B/C reuse THIS
   binary. `reports/census/unbound/PROVENANCE.md` + NOTES.md.
3. **audit-freshness.sh wired into CI** (068b37e): new "Audit
   freshness" step, green on first run (run 43091841042). Its first
   live execution caught REAL drift: cmdguard v5.0.0→v5.1.0 in go.mod
   (daemon-swept fff1f47 16:30, sibling lane). Audit re-verified
   against the tag (Go delta = +101 non-test lines: the two previewed
   opt-ins, mutex Builder, doctor doc-fixes; no cited API changed) →
   pin bumped to v5.1.0 with evidence note. CI green.
4. **Census journal snapshot routine** (04efbfd + daemon b7b63a6):
   `scripts/census-snapshot.sh` — sqlite online backup (WAL-consistent
   under load — proven by snapshotting DURING the live scan),
   integrity_check gate, tar.gz + sha256 sidecar. Restore-verified
   end-to-end (205 MB→33 MB, untar, integrity ok, meta_stream_log
   72,510 rows). Wave runner auto-snapshots on BOTH exit paths
   (stub-tested PASS + FAIL).
5. **Outage-window CI runs eyeballed** (§f12): b22b6fc, c6dca0c,
   3b3ae41, 92432b0 — all steps success, nothing skipped/masked.
6. **Soak cron verified** (§f11): 2026-11-11T03:21Z scheduled run
   42952246214 green (14m39s).
7. **Status report 22-05 + AGENTS.md gotchas** (2c27c3e, 0087584):
   wave-A completion protocol documented; gotchas added (inode-safe
   script swap, VCS-stamp provenance, online sqlite backup).
8. Session-start verification, todo reconciliation, owner-§g check
   (no answers → defaults in force).

## b) PARTIALLY DONE

1. **Wave A pass 1** — 93,249/100,000 domains scanned (93.2%),
   unknown 1,422 = 1.52% (under the 2% D3 bar all pass), unbound
   healthy, ETA ~05:20 CET. Wrapper self-manages gate + cleanup after.
   Completion protocol steps NOT yet done (scan still running):
   manual snapshot (old-inode wrapper), T11 wave-A strike, wave B
   launch. See §f.
2. **Input-accounting correction** — discovered tonight, only partly
   investigated: manifest tranco1m-a.txt = **100,000 raw = 100,000
   unique** lines, NOT the "90,834" carried by three prior session
   reports (12-35, 14-44, 15-27) and my ETAs all day (~10% error in
   every progress percentage; rate ~102/min not ~94). Unknown-rate
   verdicts unaffected (computed off actual scanned counts; both
   denominators < 2%). Root of 90,834 unestablished — needs a journal
   query (runs-table? prior session's arithmetic?) before E17 quotes
   any number.
   <!-- RESOLVED 2026-11-12 18-40 report: no journal/manifest quantity
   reproduces 90,834 (per-run observations, distinct domains, overlaps
   all checked); first appeared underved in the 14-44 report and was
   copied onward. Not a real number. -->
3. **26 public-suffix normalization artifacts** in the wave A output:
   jsonl contains bare public suffixes (edu.cn, gob.ec, gob.pe,
   ba.gov.br, altoadige.it, …) that appear NOWHERE as manifest lines —
   almost certainly www-strip reducing rows like `www.edu.cn` below a
   registrable domain. Found, counted, NOT diagnosed row-by-row (need
   the exact source rows) — input-lane wart (sibling's
   internal/input), flagged not touched.
   <!-- RESOLVED 2026-11-12 18-40 report: confirmed www.<public-suffix>
   input lines (26 of 32 www-lines in tranco1m-a) normalize to bare
   suffixes; TODO row filed (input/www-strip), sibling lane. Part of
   the same reconciliation that exposed the 2,731 resume hole. -->

## c) NOT STARTED (this session; deliberate)

- E17 census report (needs wave C + the pinned excluded pilot
  `01M74VNNCJS6`).
- Wave B/C launches (single-flight: wave A still running).
- 7 s vs 10 s timeout soak (§f13) and w2 sustained soak (§f14) —
  BLOCKED by the single-flight law while any wave runs.
- Nightly wave-heartbeat alert job (§f35) — needs an alert channel
  (owner; see §g2).
- audit-freshness into docs-gates (wired into CI only — the docs-gates
  config surface wasn't located this session; CI covers the drift case).
- Cli-lane tail items (E29 CSV enrichment, TimeoutMiddleware, describe
  contract test, interrupted-run summary fix) — sibling lane, untouched
  by design.

## d) TOTALLY FUCKED UP (honest ledger)

1. **The 90,834 input-count fiction**: I repeated it in the 22-05
   report (60.9%/61.2% "of 90,834") without ever running
   `wc -l` on the manifest — the one cheap check that would have
   exposed it. I verified everything else from primary sources tonight
   but inherited this number on trust. Three reports + all session
   ETAs were ~10% off.
2. **Progress-test harness shipped broken**: first harness defined
   `progress()` but never CALLED it — "no output" initially read as
   ambiguous before I spotted the missing invocation; then my case-1
   data was mislabeled (2 unknown/100 labeled "3%") — the CODE was
   right, my test arithmetic was wrong. Two wasted rounds on a
   10-line function.
3. **log() format bug introduced then caught**: the rewritten wrapper
   gained a third `%s` (trailing-space in every monitor.log line) —
   caught only in the pre-install diff. Should never have been typed;
   diff-before-install saved it.
4. **`rm -f` used in stub cleanup** (test one-liner) — violates the
   house trash rule. No damage (temp files), but a rule is a rule.
   Also trashed the sqlite extract BEFORE finishing verification
   queries → forced a second snapshot round to complete the proof.
5. **sqlite3 syntax fumbles** (dot-command joined to SQL; double
   quotes for string literals) — my errors, and they cost the clean
   first-try verification.
6. **Attribution split by daemon race**: census-snapshot.sh + census.md
   got swept into daemon commit b7b63a6 while I tested between write
   and commit; my commit then carried only part. Content all landed
   (verified), but the commit message understates its own diff. Same
   class as the `.census-wave.sh.new` sweep (needed a cleanup commit
   ffcbcb3). Learn: commit IMMEDIATELY after write; test after.
7. **gh `--commit` flag misuse** → empty results, retried via headSha
   matching. Small, self-caught.

## e) WHAT WE SHOULD IMPROVE (durable, from tonight)

1. **Never inherit a number** — manifest sizes, bar percentages,
   input counts: one `wc -l`/`awk` from primary source before it
   enters a report or an ETA. (Would have killed d1.)
2. **Test-first for shell deltas**: extract-and-run the REAL function
   (sed-extracted, no drift) BEFORE installing — tonight that worked
   well; the failure was my harness, not the method. Keep the method,
   add "harness must print the expected line" as its own assertion.
3. **Inode-swap is now the documented way** to edit any script a
   running bash holds (AGENTS.md gotcha added). Consider a
   `scripts/install-safe.sh` helper if this recurs.
4. **Commit-window discipline**: write → stage → commit, THEN test
   (the daemon sweeps within minutes; tonight it beat me twice).
5. **Input normalizer**: www-strip should not reduce below a
   registrable domain (26 bare public suffixes in wave A). Input lane
   (sibling) — file a TODO row rather than touching internal/input.
6. **docs cadence red is worsening by design**: every session adds a
   report (now 20 unarchived, threshold 15) while the archive ritual
   awaits owner permission — the gate will stay red and get redder
   until §g3-class decisions are made. Either raise the threshold or
   green-light the ritual.
7. **Wrapper hardening for B/C**: pass-1 jsonl line-count vs manifest
   unique-count assertion in census-gate.sh would have caught the
   90,834-vs-100,000 discrepancy automatically at the gate.

## f) NEXT (ranked; ⏳ waits on wave A completion ~05:20, 🔀 sibling lane)

1. Wave A completion protocol: verify monitor.log gate verdict; manual
   snapshot (`scripts/census-snapshot.sh … tranco1m-a-complete` — old
   wrapper inode has no auto-snapshot).
2. Launch wave B (single-flight check first; command in 22-05 report;
   wrapper mkdirs out-dir, auto-snapshots, warns above bar).
3. Strike T11 wave-A half in TODO_LIST.md (pre-flight; token: T11).
4. Root-cause the 90,834 figure (journal runs table vs prior reports)
   BEFORE E17 quotes denominators; correct census.md wave ledger.
5. Diagnose the 26 public-suffix rows end-to-end (which manifest lines
   normalize to edu.cn etc.); file input-lane TODO row (token: www-strip).
6. ~~Add manifest-unique-count assertion to census-gate.sh (see e7).~~ done at `8604fd2` (census-gate --manifest/--expect-skip, exit 4 on gap; reproduced wave A's 2,731 exactly)
7. E17 census report after wave C (pin excluded pilot `01M74VNNCJS6`). routed — the T15 census report reports/census/REPORT.md (2026-11-13; pins the excluded pilot; final distribution after wave C)
8. Wave C launch after B (T13).
9. 7 s vs 10 s timeout soak (§f13) — only between waves or after C.
10. w2 sustained soak (§f14) — only if a wave drifts above bar.
11. Nightly wave-heartbeat alert job (§f35) — blocked on §g2 channel.
~~12. audit-freshness into docs-gates proper (CI-only today).~~ **Won't implement — docs-gates was extracted as a standalone binary with its own gates; embedding it into buildflow would duplicate the battery.**
13. Census db snapshot rotation policy (snapshots dir will grow ~33 MB
    per wave — fine for 3 waves, needs a policy by E17).
14. E17 export: compact verdict table + `replay --since` demo (§f31).
15. README: document `replay --since` (§f32, under-marketed).
16. Verify tonight's 03:17Z soak run green (third scheduled fire).
17. Monitor wave B with the NEW wrapper features (WARNING lines,
    auto-snapshot) — first live exercise of both.
18. Archive ritual for 20 unarchived reports — BLOCKED on §g3.
    <!-- fresh:skip owner-blocked archive ritual, §g3 of 15-27/04-15 -->
19. 🔀 cli lane: interrupted-run stderr summary fix (TODO row filed).
20. 🔀 cli/scanstore: runs-table metadata shutdown race (soak-3 row).
    <!-- fresh:skip sibling-scoped pending owner policy -->
21. 🔀 E29 CSV enrichment; TimeoutMiddleware; describe contract test.
22. 🔀 Harvest sales-lane 19-27 §f (18 items) — owner policy §g1.
23. 🔀 Harvest E24 02-00/19-08 §f (8 items) — same.
    <!-- fresh:skip sibling-scoped pending owner policy -->
24. flake vendorHash vs cmdguard v5.1.0: TODO row already struck [x]
    by the bumping lane — verify once with `nix build` when a lane
    next touches nix (do not take the strike on trust; see e1).
25. IPv6 --resolver handling check (§f25).
~~26. TXT truncation investigation (existing TODO row).~~ **Superseded — closed with live evidence (00-03 §8).**
27. E28 --pre-filter + .ch SWITCH AXFR tooling (existing rows).
28. crt.sh .de rate-limit-aware scheduling (routed earlier).
29. reports/ durability policy beyond local tarballs (§g3).
30. Sibling 17-43 cqrs-event-sourcing-audit-status: read + route its
    §f when its lane goes quiet (policy §g1).

## g) QUESTIONS (cannot answer myself)

1. **The three standing §g questions from 15-27 remain unanswered**
   (w4-throughout vs funding an offsite resolver; cross-lane
   harvesting policy; snapshot destination beyond local tarballs).
   Defaults remain in force — confirm, or correct any of them now
   that wave A will have proven the w4/10s two-pass protocol
   end-to-end?
2. **Alert channel for census waves**: a stalled wave currently pages
   nobody (§f35). Where should a heartbeat alert GO — there is no
   channel I can pick myself (email? a nextd queue item? a file the
   next session must read? GitHub issue auto-open requires a token
   decision)?
3. **May I run the archive ritual?** 20 unarchived reports vs
   threshold 15, red and worsening each session. The ritual bulk-moves
   SIBLING-authored reports into docs/status/archived/ with
   live-remainder banners. Lane courtesy says ask; the gate says now.

---

**Live state (04:15 CET):** wave A 93,249/100,000 (93.2%), unknown
1,422 = 1.52%, unbound healthy, pids 91698/92164/92166 alive, ETA
~05:20, then wrapper gate (+cleanup if FAIL). Tree clean at 0087584,
CI green on the last three pushes, all commits on both remotes.

**WAITING FOR INSTRUCTIONS** (wave-A completion protocol will execute
per the documented default if no countermand arrives before ~05:20).
