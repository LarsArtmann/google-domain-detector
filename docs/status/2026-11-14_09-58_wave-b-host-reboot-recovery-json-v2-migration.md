# Status: json-v2 test migration closed, gate green — then the host rebooted and wave B died mid-pass; recovered as attempt 3

**Session:** 2026-11-14 08:46 → 09:58 CET · **Mandate:** "READ, UNDERSTAND, RESEARCH, REFLECT … Execute and Verify one step at the time … until done" (resume of the 03-18 WAIT state; the 3 owner questions from 03-18 remain UNANSWERED and were treated as gates, not blockers for unblocked work)

**Session start state:** tree clean at `18448b7` (both remotes), CI green ×3 (incl. Soak), wave-B attempt 2 alive (pid 2327170, 15h53m), single-flight = 1.

---

## a) FULLY DONE

1. **Session-start checks** — git/CI/single-flight/heartbeat all green at 08:46; wave-B attempt 2 at 214,634/303,785 (~4.0/s), NXDOMAIN share 2.27% (existence-oracle fix holding), watchdog silent.
2. **Round-2 plan #17 — full buildflow gate re-run after the glossary landed**: fresh run (`BUILDFLOW_NO_RESULT_CACHE=1`) completed with all remaining warns being the documented non-fixes (go-version-auto-configure vs cqrs floor, vulnix nixpkgs CVEs, lychee 403, shellcheck on frozen census scripts, nix-checker vendorHash). **Exit 0 confirmed** via the documented cached-replay pattern (`GATE_EXIT=0`). Exit code note: the FIRST run's exit code was lost to a `| tail` pipe and had to be re-confirmed by a second cached run — see §d.
3. **Round-2 plan #19 — json-v1 audit findings decided and FIXED at source.** The 26 findings decomposed as: 4 json-v1 tail findings (3× `third-party-json-semantics` conservatism + 1× `import-rewrite` + 1× `duration-field` trap) + 21 `suggest-lo-*` style hints + 1 `lo-dependency-missing`. Analysis: all four flagged test files (audit_test, cli_scan_test, cli_scan_stream_test, provenance_test) have string-only `any`-sink reads and the flagged `hangRes.holdAfterCancel time.Duration` fixture field is **never serialized** — so the tool's conservative skip (v2 SemanticError on Duration without `format:nano`) does not apply to actual usage. Migrated all four imports to `encoding/json/v2`. Verification: `go build ./...` + `go test ./internal/cli/ ./internal/scanstore/` green, gofmt clean, audit re-run **v1 rules 26→0** (21 lo hints remain).
4. **The 21 `suggest-lo-*` findings recorded as DOCUMENTED NON-FIX**: samber/lo is not a go.mod dependency (only samber/do is, indirect); adding it for style sugar is a stack decision (stack-selection territory), not hygiene; several hints themselves say "rewrite not proven safe".
5. **03-18 report items 17 + 19 annotated inline** (repo ANNOTATE convention) with evidence and the non-fix rationale.
6. **Commit + push + CI green**: annotation landed as daemon commit `7e94bc2`, amended message-only to `c75dd1d` (byte-identical tree — policy-legal), pushed to BOTH remotes; **CI success on `c75dd1d`** confirmed. The 4-file migration itself rode daemon commit `5ad9b1b` (clean bundle, only my 4 files) — see §d for the message-quality cost.
7. **crt.sh 04:37 timer verification** (was time-gated, became due): the 04:51 run **landed in-repo** (the wrong-root path fix from 2026-11-13 verified working) and the **plausibility guard correctly caught a truncated .de harvest** (2,060 rows < 100k floor, artifact removed, logged). Same exact 2,060 count as yesterday's truncation — systematic crt.sh-side truncation, not random. A sibling session independently recorded the run in TODO_LIST/census.md (`3a76e78`) — no conflict.
8. **LSP ghost warnings flushed** (restart): classify.go "undefined isMicrosoftMX" etc. were stale — the M365 matchers live in `matchers.go` (file-size pre-split `048c824`, green on master).
9. **INCIDENT RECOVERY — wave-B attempt 3 launched and healthy.** Full narrative in §d/§e context; end state: pre-flight PASS over TCP, pid 634513, explicit resume, ~4.8/s, ETA ~07:00 Nov 15.
10. **Incident durably recorded**: incident line in wave monitor.log + archive copy of attempt-2 log at `/tmp/wave-b-monitor-attempt2-archive.log` (the wrapper truncates monitor.log on every relaunch) + full narrative in `docs/census.md` wave table, committed `b9604e3`, pushed both remotes.
11. **Heartbeat cadence re-established** post-recovery (next checkpoint due ~10:25).

## b) PARTIALLY DONE

1. **crt.sh .de truncation root cause**: mechanism (path + guard) verified working; the TRUNCATION itself not investigated (2,060 rows two days running = systematic; crt.sh rate-limit/overload suspected). External service — parked.
2. **Host monitoring degradation**: gatus fired real alerts (system_health.prom rejected by node_exporter → all system_* metrics blind; smartd dead; nix-daemon oomd-killed; monitor365 crash-loop; PMA at 90% of 16G ceiling). Post-reboot ps shows smartd/nix-daemon/monitor365 RUNNING again; **system_health.prom status NOT verified** (out of repo scope, needs node_exporter journal check).
3. **Wave B**: attempt 3 in flight (956 lines at 09:58, ~4.8/s, healthy) — NOT yet complete, gated, or snapshotted. ETA ~07:00 Nov 15.
4. **json-v1 hygiene**: production code was already v2; test tail now fixed — but the audit step permanently emits the 21 documented lo hints, so every gate tail still carries that noise.
5. **Attempt-2 provenance in the ledger**: the census.md row records attempt 2's death and the re-scan-by-design decision, but attempt 2's actual run id (discoverable from the journal) was NOT extracted into the record — future `replay --audit-excluded` work will need it.

## c) NOT STARTED

(All previously identified; unchanged, still gated or wall-clock-gated.)

1. **Owner-gated 8-ask package** (docs/planning/2026-11-14_02-18-owner-decision-package.md): RulesetV2 stamp (`v2-2026-11-14`), wave-C launch policy, poisonfix-25, www-strip rescan-slot, GPG signing policy, BuildFlow#56 fleet rebuild, Nominet .uk email, M365 surface+DKIM probe.
2. **Post-B chain** (~07:00 Nov 15): wrapper auto-gate verification (`--expect-skip 96,215`), snapshot bundle re-check, T14 cleanup loop IF gate FAILs (policy = owner question), wave C, poisonfix, T15 final distribution, closure report.
3. **#18 shellcheck SC2034** in census-gate.sh/census-wave.sh — frozen until B's gate completes (post-B, new-inode install).
4. **#21 go-cqrs-lite patch sweep re-check** — post-census.
5. **#20 scanstore_test.go pre-split** — conditional (at 328/350; act before any addition >22 lines).
6. **Parked**: asciinema demo, TCP-contention analysis, zone-ch scan launch prep execution (docs ready), CRM delta retirement confirm.

## d) TOTALLY FUCKED UP

Nothing caused data loss or code damage — but three real process failures, honestly:

1. **The host reboot killed wave-B attempt 2 and I detected it ~40 minutes late.** The machine died ~09:30 (memory-pressure storm → oomd kills 09:19 → reboot). My heartbeat discipline said every ~30 min but I absorbed the interval into repo work and only checked at 09:49. Recovery was barely delayed (nothing could restart before host stabilization anyway), but the detection gap is exactly the failure mode the cadence exists to prevent. Worse: **the 2026-08-22 "freeze precursor profile" fired its alert at 09:14 and the monitoring that should have caught it was BLIND** (system_health.prom rejected) — the storm escalated to a full reboot with no early-warning response. (Host-ops fault, but census ops rode the blast.)
2. **The v2 migration commit carries a junk daemon message permanently.** My edits sat uncommitted ~5 min while I verified tests; the daemon swept them into `5ad9b1b chore: auto-commit 4 changed file(s) (heuristic)`. By the time I went to commit properly it was already bundled; the amend I COULD do (annotation `7e94bc2`→`c75dd1d`) was legal only because that one was still unpushed. `5ad9b1b` was pushed before I noticed. Lesson: on this host, commit immediately after each verified edit unit — verification-first cost me the message.
3. **Lost the gate exit code on the first full-gate run** (`buildflow 2>&1 | tail -60` — $? was tail's). This exact gotcha is IN my notes from the previous session ("use no-pipe or PIPESTATUS") and I did it anyway. Burned a second (cached) run to confirm exit 0. Also the pipe buffered ALL gate output until completion, giving zero mid-run visibility.

Related near-miss (not quite fucked up, worth logging): I launched attempt 3 while host load average was 1034 and CLIMBING (boot storm, 21 users). Memory was recovering (17→38 GiB available) so I judged the memory-light scan safe — correct in hindsight, but I never identified WHAT was generating load 1034 (30 seconds of `ps aux --sort=-%cpu` would have answered it).

## e) WHAT WE SHOULD IMPROVE

1. **Heartbeat cadence must be time-anchored, not work-anchored** — the 30-min poll should fire on wall-clock checkpoints even mid-task (lesson d1).
2. **Commit-with-message immediately after each verified edit unit** when the daemon is live; treat the verify-then-commit window as a race (lesson d2).
3. **Never pipe buildflow through `tail`** — redirect to a file and read it, or use PIPESTATUS (lesson d3, recorded twice now).
4. **Recovery runbook addition (census.md)**: before ANY wrapper relaunch, archive `monitor.log` + `run1.jsonl` metadata — the wrapper truncates both on launch and the pre-reboot forensic record is otherwise lost.
5. **`unbound.sh status` lies after a reboot** (reported "not running" while pid 523603 was serving NOERROR) — its check should probe like the wrapper's pre-flight does (dig-based); small post-B script fix on a NEW inode.
6. **gdd `--resume` enhancement candidate**: resume accepts ONE run id, so attempt 2's ~228k superseded observations are re-scanned (~16 h wasted at 4/s). A "skip union of run ids X,Y" or wave-scoped skip would make crash resumes nearly free. Changes scan CLI semantics → owner decision.
7. **Multi-attempt gate arithmetic**: the gate's `--expect-skip` counts only the ORIGINAL run's journal domains; superseded partial runs ride in the journal forever as orphans. Consider a wave-level reconciliation (sum over the wave's run ids) or explicit supersession marking.
8. **Monitoring-of-monitoring**: the system_health.prom textfile collector was rejected at the exact moment its data mattered. node_exporter parse failure should page louder/faster than the metrics going dark did.
9. **Host memory hygiene**: 2026-08-22 freeze-profile storm recurred. GPU GTT at 60G + PMA at 90% ceiling + oomd kills — the host needs a memory-budget review that monitoring alone won't deliver.
10. **crt.sh truncation**: identical 2,060-row .de harvest two days running — worth a direct API probe (count/limit params) before trusting the 04:37 feed for zone-ch planning.

## f) UP TO 50 THINGS WE SHOULD GET DONE NEXT

**Census chain (the critical path):**
1. Heartbeat attempt 3 every ~30 min until ~07:00 Nov 15 (single-flight = 1, jsonl growing, watchdog silent).
2. On B exit: verify wrapper auto-gate PASS (`--manifest --expect-skip 96215`) + snapshot bundle.
3. Extract attempt 2's run id from the journal into the census ledger (audit provenance).
4. Post-B snapshot bundle re-check on B's OWN snapshot (the 1/5 remainder from §b3 of the 03-18 report).
5. Owner answer #1 → stamp RulesetV2 `v2-2026-11-14` (10-min plan: ruleset.go const + pin test + classify tests) BEFORE wave C's binary.
6. T14 cleanup loop if B's gate FAILs (per owner answer #2).
7. Owner answer → wave C launch (500,001–1,000,000, ~6.6 h ahead of B's pattern; single-flight law).
8. poisonfix-25 (fresh binary, PROVENANCE stamp, 25-domain list) — owner answer #3.
9. Closure proof: `replay --audit-excluded <4 run IDs>` → 0 poisoned.
10. T15 final whole-census distribution in REPORT.md (label Microsoft-evidence runs per the ruleset question).
11. Census.db headroom re-check for wave C + zone-ch (~10.5 GiB vs 215 G free at last check).
12. `--resume latest` hazard: consider a hard warning/alias removal in gdd now that it bit wave A (2,731 domains) and the wrapper works around it — CLI polish.
13. #18: shellcheck SC2034 in census-gate.sh + census-wave.sh (post-B, new-inode install).
14. #21: go-cqrs-lite patch sweep re-check post-census.
15. #20: scanstore_test.go pre-split before any addition >22 lines.
16. crt.sh .de truncation investigation (API probe; identical 2,060 twice).
17. zone-ch launch prep EXECUTION (separate zone-ch.db, 6×450k batches ≈8 days, pilot-gate 10k throwaway — plan ready in census.md §R3).
18. Nominet .uk data path decision (owner ask; draft ready in docs/zone-data/nominet-uk.md).
19. Post-census: asciinema demo recording (parked).
20. Post-census: TCP-contention analysis (parked).
21. Post-census: CRM delta retirement confirm (CRM consumes `replay --since`; T16 landed).
22. gdd resume-enhancement design (multi-run skip union — §e6) — owner decision, then implement.

**Repo hygiene / gates:**
23. Watch CI on `b9604e3` (docs-only, but the release law still applies to anything tagged).
24. CI on sibling's `476a52a` harvest commit was in_progress at 09:49 — confirm green (their lane, report only).
25. BuildFlow#56: pre-commit hook still fails valid commits — owner ask #6 (fleet rebuild approval); until then `--no-verify` + direct gates.
26. Decide json-v1 for the REMAINING `encoding/json` import sites if any appear (none known — verify zero left after this session's fix).
27. docs-gates freshness on the two annotations (verify the 03-18 report passes gates with the appended DONE blocks — it was committed `--no-verify` as part of the daemon flow).
28. Re-run `buildflow -s go-auto-upgrade` once post-census to confirm the lo-hint set didn't grow with new code.

**Host ops (report + optionally repair with approval):**
29. Verify system_health.prom parse failure root cause (node_exporter journal) — the alert gave the exact command.
30. Confirm smartd stable post-reboot (ps shows it up; the 2026-08-18 silent-death class precedent says verify, don't assume).
31. Swap posture: 48G/62G used post-reboot — decide drain or leave.
32. PMA (auto-commit daemon) memory: hit 90% of its 16G MemoryMax during the storm — review its MemoryMax/MemoryHigh budget.
33. Identify what drove load 1034 in the boot storm (unanswered §d item).
34. GPU GTT 60G alert — decide if action needed or alert threshold tune.

**Docs / provenance upkeep:**
35. STATUS index row for this report (done with the commit, but listed for completeness).
36. PROVENANCE.md: gdd-20261113b sha256 verified this session — no action needed; keep the stamp-first discipline for the wave-C binary.
37. census.md: post-B row update for tranco1m-b (attempt 3 outcome + final counts) — after the gate.
38. census.md: post-C row + whole-census totals — after wave C.
39. DOMAIN_LANGUAGE.md: add "superseded pass" and "attempt" terms (used heavily in this incident; currently absent).
40. TODO_LIST.md: strike the json-v1/round-2 rows if any reference remains stale after the sibling's harvest.
41. AGENTS.md is at 219/220 — no headroom; the reboot/forwarder lessons live in census.md (correct home). Consider whether the census gotcha block should point to census.md §incident instead of duplicating.

**Owner-gated package (repeat for visibility):**
42. Ask #1: RulesetV2 stamp + pre-authorized B→gate→C→poisonfix chain.
43. Ask #2: T14 auto vs hold on gate FAIL.
44. Ask #3: GPG signing policy for agent commits.
45. Ask #4: www-strip rescan-slot in wave C (policy decided; only the slot is live).
46. Ask #6: BuildFlow#56 fleet rebuild.
47. Ask #7: Nominet email send.
48. Ask #8: M365 surface (render tally vs Counts v2) + DKIM microsoft selector probe — park vs plan.
49. Wave-C binary build + PROVENANCE stamp (after RulesetV2 decision — same binary decision point).
50. Whole-census closure: final REPORT.md, closing status report + index row + push (the census closeout ritual), then the parked items 19-21 unblock.

## g) THREE QUESTIONS I CANNOT FIGURE OUT MYSELF

1. **RulesetV2 stamp + chain pre-authorization** (carried from 03-18, ask #1/#2 of the package): Do I stamp `v2-2026-11-14` now so wave C's binary carries it, and do you pre-authorize the B→gate→C→poisonfix chain to run unattended overnight (B gate ETA ~07:00, so the decision gates tomorrow morning's work)?
2. **T14 on FAIL** (carried, ask #2 core): if B's coverage gate FAILs (exit 4, and note attempt 2's superseded-pass arithmetic is the one new risk factor), do I auto-run the T14 cleanup loop or hold for your go?
3. **The 09:15 storm attribution + host-repair scope**: gatus says the nix daemon was "likely killed by systemd-oomd during a build" — was that YOUR build (i.e. is the storm explained and over), and do you want repo-session agents repairing host services (system_health collector, swap posture, PMA budget) or is host ops out of our lane entirely?

---

**Pushed this session:** `c75dd1d` (report annotations + closes 03-18 items 17/19), `b9604e3` (census.md incident record) — both remotes, CI green on the code commit. **Daemon commits carrying my content:** `5ad9b1b` (v2 migration, junk message, permanent), `7e94bc2`→amended. **Sibling sessions live**: `3a76e78` (crt.sh record + LSP row), `167b230` (projectionhost issue draft), `476a52a` (docs harvest).
