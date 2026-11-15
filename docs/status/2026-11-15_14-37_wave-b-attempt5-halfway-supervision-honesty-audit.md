# Wave B attempt-5 supervision to halfway + honest failure audit — 2026-11-15 14:37 CET

Session: 07:35 → 14:37 CET. One operator: this session (post-handoff from the
07:30 report). No owner instructions arrived, so the standing mandate ran:
PSI-gated attempt-5 launch, supervision beats, honest records, owner-gated
items untouched.

## Machine state at 14:37

| Thing            | Value                                                                                                                                                                                                                  |
| ---------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| HEAD             | `dd72b05` (daemon table-padding sweep) on top of `ba21f99`; origin in sync, github mirror 1 behind (pushing with this report)                                                                                          |
| CI               | green on every commit this session (77e3065, 72e9d3c, ba8017b, ba21f99)                                                                                                                                                |
| Wave B attempt 5 | **LIVE since 07:50:52**, 91,579/171,584 scanned = **53.4%**, avg 3.75/s → **ETA ~20:33 CET**                                                                                                                           |
| Resume target    | attempt-2 run `01M7ASTM63TVRJ6RXCGR00EYVQ`, gate expect-skip **228,416** (journal-derived at launch)                                                                                                                   |
| Health           | single-flight=1 in every beat; memory PSI avg10 0.08-3.03 (quiet); watchdog alarms **0**; poller warnings stuck at 2 (both early, 2.42%/2.01% — declining since); unknown rate now **1.52%** (band-calibrated decline) |
| Forwarder        | unbound-tcp pid 1281316 alive the whole session; upstream TCP conns sampled 16-53 (cap 150 — never contended)                                                                                                          |
| Reboots          | ZERO since 02:25. The doom loop did not fire today                                                                                                                                                                     |

## Timeline of this session

- 07:35 survey: CI on `77e3065` confirmed green (release law). PSI decaying
  70% → 2.98 → 11.06 (bounced) → 4.71 → 5.04 → launch gate passed.
- 07:38 forensics archived FIRST (/tmp pre-attempt5: run1.jsonl 61 MB,
  run1.stderr, monitor.log).
- 07:44 forwarder restarted (`GDD_UNBOUND_CONF=scripts/unbound-tcp.conf
  scripts/unbound.sh start`, pid 1281316); dig +tcp probe NOERROR; 5353
  refusal understood — the TCP conf binds only 5354 (DOMAIN_LANGUAGE's
  "5353" is the generic description, not this variant's port).
- 07:48 **launch attempt 1 FAILED** (env prefix dropped — see §d1). Preflight
  refused, exit 5, zero side effects, run-id untouched.
- 07:50 **attempt-5 LAUNCHED** with the full env; all six verification lines
  present (TCP preflight, 2× upstream, forwarder, pass-1 start, resume line,
  expect-skip = 228,416 exactly); watchdog attached (pid 2291507); PSI watch
  loop printed `PSI-CLEARED`.
- 07:52 census.md ledger appended (attempt-4 death by reboot #3 at 140,981 +
  attempt-5 launch + the 07:48 refusal incident); docs-gates green; `72e9d3c`
  pushed both remotes; CI green.
- 08:20-14:12 nine supervision heartbeats: rate 3.64-4.2/s (steady ~3.8),
  PSI 0.08-3.03, TCP 16-53, alarms 0, single-flight=1 always, no sibling
  writes. Unknown rate declined 2.42% → 1.52% through the rank bands exactly
  as calibrated.
- 08:2x-09:0x H2 prep: shellcheck on all 7 census scripts — 2 warnings, both
  root-caused (§a), fixed copies staged in /tmp, shellcheck+bash -n clean.
- 08:2x-11:5x TCP contention measurement (§a5, §d2) → NOTES.md + census.md
  verdict + TODO struck (`ba8017b`).
- 08:23-12:30 R4 demo recorder: script → committed (daemon `bdf7945`);
  renamed to `demo-record.sh` (git mv, `d92c2d9`); agg attr wrong → reworked
  to vhs 0.12.1 (`ba21f99`); full vhs+ttyd+ffmpeg chain smoke-tested with a
  no-scan tape.
- 14:01 daemon `dd72b05` (census.md table padding, flake.lock, minor script
  formatting) — inspected, benign, left as-is per mixed-commit precedent.
- 14:2x C2 exit mechanics pre-verified (§b4).

## a) FULLY DONE

1. **Attempt-5 launched and healthy at halfway** — PSI-gated, preflight-
   verified, expect-skip 228,416, workload on schedule (53.4%, ETA ~20:33).
2. **Attempt-4 death + reboot #3 recorded** in the census.md wave ledger
   (`72e9d3c`).
3. **The 07:48 launch-refusal incident recorded** — the guardrail worked;
   documented as a worked-example in the ledger.
4. **H2 root-caused** (read-only while the wrapper lives): census-wave.sh:50
   `bar=` is a dead duplicate (census-cleanup.sh reads the env itself — the
   wiring is NOT broken, delete the line); census-gate.sh:148 `errors` is a
   dead read-target (the stderr summary form already discards the errors
   total at line 134 — dropping the field makes the jsonl form consistent,
   7→6 fields). Fixed copies staged in /tmp, shellcheck+bash -n clean.
5. **TCP cap contention question ANSWERED** (TODO row struck): 150 never
   contended at w8 — upstream conn population 17-44 at w4 (linear w8
   scaling ≈ 34-88, ≥ 40% headroom); `incoming-num-tcp` unused in this
   topology (scanner hops UDP, 0 client-side TCP in every sample); fd
   pressure 134/65536. Raw data + the measurement gotcha in
   reports/census/unbound/NOTES.md; durable verdict in census.md (`ba8017b`).
6. **R4 demo recorder ready and committed**: `scripts/demo-record.sh`
   (single-flight guard, 60-domain slice, asciinema cast + vhs GIF with
   Sleep 25s so the GIF shows completed output; TypingSpeed tuned; em dashes
   out of source strings). Toolchain verified end-to-end via a no-scan
   smoke tape (`ba21f99`).
7. **CI green on every pushed commit**; release law held all session.
8. **Zero alarms, zero reboots, zero stall events** across ~7 h of live
   supervision; the detached stack (wrapper poller + watchdog) independently
   survived a 2 h session-idle gap (09:54→11:52) without missing a progress
   line.
9. **Daemon bundles inspected** (bdf7945, d92c2d9, dd72b05): all benign
   formatter/rename sweeps, left as-is per mixed-commit precedent.

## b) PARTIALLY DONE

1. **C1 supervision**: wave at 53.4%, beats continue until exit (~20:33).
   Everything pre-staged for a clean finish.
2. **H2 fixes**: root-caused + staged in /tmp, but NOT installed — census
   scripts are frozen while the attempt-5 wrapper and watchdog live (new-
   inode install at B exit, then re-shellcheck + commit).
3. **R4 execution**: script + toolchain ready; the REAL recording (spawn a
   scan) must wait for single-flight-free; GIF → README embed after that.
4. **C2 pre-verification** (mechanics, not execution): stderr stays empty
   during the run and fills at completion (summary + `run <id> recorded`);
   the wrapper greps the run id → writes run-id → runs the gate on
   run1.stderr; PASS → snapshot `tranco1m-b-complete-<ts>.tar.gz` + sha256 →
   exit 0; FAIL → cleanup loop → snapshot → exit cleanup_rc; coverage gap →
   exit 4. Execution happens at B exit.
5. **Mirror sync**: github is 1 commit behind origin (daemon `dd72b05`);
   pushing with this report.

## c) NOT STARTED

- C6 closure proof (supersession audit = 0 poisoned), C7 T15 distribution
  (CRM consumes `replay --since` deltas), C8 closing report, R6 docs-health
  pass — all gated behind B's exit.
- Owner-gated set untouched (§g below + the 02-18 8-ask package): skip-union
  implementation, pause-vs-push-through decision, unattended chain, RulesetV2
  freeze, poisonfix GO, www-strip slot, GPG, BuildFlow#56 rebuild, Nominet,
  M365 surface.
- A REAL w8 TCP contention sample (analytical extrapolation only so far —
  flagged in NOTES.md; a w8 sample needs a future w8 context).

## d) TOTALLY FUCKED UP (honest; all self-caught, zero permanent damage)

1. **The 07:48 env-less launch.** I dropped the `GDD_RESOLVER=...
   GDD_PREFLIGHT_TRANSPORT=tcp GDD_UNBOUND_CONF=...` prefix from MY OWN
   written ritual — reconstructed from memory instead of executing the
   handoff spec verbatim. The wrapper's preflight refused (resolver fell
   back to the 5353 default, probe failed, exit 5). Zero damage BECAUSE the
   guardrail exists — and because 5353 was dead. Had 5353 been alive, the
   env-less launch would have LANDED on the wrong resolver and poisoned the
   wave's measurements. That near-miss is the real lesson: the guardrail
   that saved me was incidental (a dead port), not designed.
2. **The unanchored-regex measurement bug.** My first TCP samples used
   `[0-9.]+:53` — matching local ephemeral ports 53xxx as "peers",
   fabricating phantom conns to 192.168.1.150:53 (this host's own LAN IP)
   and double-counting (58-vs-44 mismatch was the tell). I initially
   investigated it as a REAL anomaly (possible provenance break) before
   suspecting the ruler. Corrected in writing (NOTES.md gotcha); anchored
   parsing for all later samples. Cost: ~15 min and one false alarm.
3. **`nixpkgs#agg` assumed, not verified.** I wrote "convert with
   nix run nixpkgs#agg" into the COMMITTED demo script before checking the
   attr — nixpkgs `agg` is an unrelated C graphics library. Reworked to
   charmbracelet vhs (`ba21f99`). Verify tool availability BEFORE it enters
   a committed artifact.
4. **Em dashes in source strings** shipped in the first demo-script commits,
   violating the explicit no-em-dash-in-source rule; caught on self-review,
   fixed via amend (`ba21f99`).
5. **Two stale-read edit failures** (census.md, demo-record.sh after the
   git mv) — pure round-trip waste; the fresh-view-before-edit discipline
   after file changes must be automatic, not re-learned each time.

### What did I forget? (direct answer)

The env prefix (d1). Also: that supervision beats only fire while I am in a
turn — the 09:54→11:52 idle gap had no beats from me; the detached stack
covered supervision, which is BY DESIGN, but my mental model of "beats every
25 minutes" was wrong for this session shape. Also: fresh views before edits
after any file change/rename (twice).

### What could I have done better? (direct answer)

Execute rituals verbatim from their written spec — or better, encode the
launch ritual into a helper script so the operator-error class disappears
entirely (see §e2). Anchor measurement regexes from sample one and
sanity-check measurement sums (the 58-vs-44 mismatch was visible early).
Verify nix attrs before committing references to them. Push both remotes in
the same command every time (the daemon's `dd72b05` rode origin-only for
~35 min).

### What could I still improve? (direct answer)

See §e — the structural items: formalize the PSI gate in the wrapper, add
the env-coupling guard, implement the skip-union (owner-gated), and give the
mid-run-death resume-target upgrade a documented manual step (§e6/f29).

## e) WHAT WE SHOULD IMPROVE

1. **Encode the launch ritual.** The 07:48 error is a CLASS: any future
   relaunch reconstructed from memory risks the same drop. A
   `scripts/census-relaunch.sh` (archives forensics, restarts forwarder,
   probes, launches with the full env, verifies the six lines, attaches the
   watchdog) turns the ritual into one reviewed command. Candidate for
   post-B (its own implementation must not touch live scripts mid-wave).
2. **Formalize the PSI launch gate** in census-wave.sh: read
   /proc/pressure/memory avg10 before preflight, refuse above a
   `GDD_MIN_PSI` threshold (default 10). Today it is operator discipline
   only — it worked this time because I happened to enforce it.
3. **Env-coupling guard in the wrapper**: if the TCP variant conf is in use
   (port 5354), WARN/require `GDD_PREFLIGHT_TRANSPORT=tcp` — closes the
   d1 near-miss for the case where the default resolver port happens to be
   alive.
4. **Anchor-regex lesson** → cross-project measurement lesson candidate for
   crush-config references/lessons.md (committed there by policy, not
   in-session).
5. **NOTES.md backup**: reports/ is gitignored; the TCP measurement raw
   data lives outside git. At census close, either sha256-sidecar the
   unbound/ notes like PROVENANCE.md or copy the durable sections into a
   tracked doc.
6. **Mid-run-death resume-target upgrade**: census-wave.sh writes run-id
   ONLY at scan completion (from the stderr summary). If attempt-5 dies
   mid-run, run-id still points at attempt-2's id and a naive relaunch
   re-scans attempt-5's ~90k+ fresh observations. The manual recovery
   (query the journal for the dead run's id, write run-id, relaunch) exists
   but is undocumented — one more reboot before ~20:33 makes this live.
   Document it in census.md now; the skip-union fix (§g1) makes it moot.
7. **`unbound.sh status` still misreports** post-reboot (AGENTS gotcha,
   re-confirmed this session by relying on dig). Post-B: make status
   dig-probe internally.
8. **Watchdog PSI awareness**: census-watchdog watches stalls only; a
   memory-storm early-warning line (PSI avg10 ≥ 20% → ALARM) would turn
   reboots from surprises into 10-minute evacuations.
9. **Demo script self-review cadence**: the first committed version carried
   two fixable flaws (agg assumption, em dashes). A pre-commit self-check
   against the critical rules list would have caught both.

## f) UP TO 50 NEXT THINGS (ordered by impact; owner-gated marked)

Post-B mechanical (B exits ~20:33 — all staged, ~1 h total):

1. Verify wrapper auto-gate PASS: `--manifest --expect-skip 228416`; math
   171,584 + 228,416 = 400,000 exactly.
2. Extract attempt-5 run id (stderr + run-id file) into the census.md
   ledger completion row.
3. Verify auto-snapshot flowed: `tranco1m-b-complete-<ts>.tar.gz` + sha256
   sidecar in reports/census/snapshots/.
4. Restore-verify B's snapshot (copy out, `PRAGMA integrity_check`,
   journal count, replay smoke on the COPY — never the live db).
5. Check disk space before/after the snapshot (~100 MB scale).
6. Post-B bundle: census.md rows + TODO strikes + commit + push BOTH
   remotes + CI green.
7. Install H2 fixes via new inode (`install -m 755 /tmp/census-*.fixed`),
   re-shellcheck, delete staged copies, commit.
8. Run `scripts/demo-record.sh` (real 60-domain scan, cast + GIF).
9. Review the GIF (frame timing, table truncation via head -40), then embed
   `![](assets/demo.gif)` in README after the quickstart + commit (owner
   review question, §g3).
10. `replay --audit-excluded` supersession audit — poisoned count must
    reconcile against the poisonfix input list.
11. R2 refresh: final rank-band unknown-rate calibration from B's complete
    pass-1 data into census.md.
12. Kith CRM handoff refresh: tier CSVs from B (downstream consumer law).
13. Sales artifacts refresh (crm/docs/research + sales html) with final
    B numbers.
14. Forensics lifecycle: /tmp wave-b archives die on reboot — copy the
    durable ones (pre-attempt5 jsonl archive) into reports/ (gitignored)
    or drop with a note.
15. Verify CI on `dd72b05` (daemon commit; green expected, verify anyway).
16. Sweep github mirror sync after this report's push.

Wave C path (gated on §g2/§g3):
17. Wave C go/no-go (owner).
18. If go: verify C manifest sha (b0317d71873a pinned in the ledger),
fresh forwarder, PSI gate, launch fresh (no resume), w4/10s.
19. If go: pre-position the C watchdog + poller + GDD_WARN_PCT from the
recalibrated bands (item 11).
20. poisonfix wave (tranco1m-a-poisonfix, 25 domains) — input staged since
2026-11-13; needs B single-flight-free + owner GO (8-ask #3).
21. After poisonfix: re-run the supersession audit — poisoned must be 0
(C6 closure proof).

Structural fixes (post-B, non-owner or proposal-level):
22. `scripts/census-relaunch.sh` ritual encoder (§e1).
23. PSI launch gate formalization in census-wave.sh (§e2, GDD_MIN_PSI).
24. Wrapper env-coupling guard: TCP variant ↔ tcp preflight transport (§e3).
25. Document the mid-run-death resume-target upgrade in census.md (§e6) —
do this BEFORE B exits, it is live-risk documentation.
26. census-watchdog PSI alarm line (§e8).
27. `unbound.sh status` dig-probe fix (§e7).
28. NOTES.md backup ritual (§e5).
29. Anchor-regex lesson → crush-config references/lessons.md (commit there,
not in-session).
30. AGENTS.md: one line on the 07:48 refusal class + ritual-verbatim
discipline (durable gotcha, not just a status-report artifact).

Owner-gated (surface, never execute without answers):
31. §g1 skip-union implementation (scan CLI + scanstore semantics) —
THE doom-loop fix: any resume becomes ~30k ≈ 2.3 h, reboot-proof.
32. §g2 pause-vs-push-through for the chain.
33. §g3 unattended chain pre-authorization (B → C → zone-ch?).
34. RulesetV2 freeze decision (8-ask #1).
35. www-strip policy slot (8-ask #4).
36. GPG signing setup (8-ask #5).
37. BuildFlow#56 pre-commit hook rebuild (8-ask #6).
38. Nominet data request send (8-ask #7).
39. M365 surface + DKIM probe (8-ask #8).
40. Library audit re-audit trigger check (cmdguard/go-output drift — only
if the dep set changed; verify go.mod unchanged → skip).

Later census lanes:
41. zone-ch launch prep executes per its census.md section (separate store,
6 × ~450k batches, throwaway-store pilot) — after C + poisonfix.
42. zone-ch daily timer verification continues (04:37 in-repo proof).
43. DOMAIN_LANGUAGE.md: no changes needed this session (terms stable) —
re-check after any RulesetV2 decision.
44. Post-census docs-health: archive the 02-05/02-18/02-46 report
generation (R6.1), TODO hygiene re-run (R6.2).
45. C8 closing report (R6 harvest input).

Hygiene:
46. Delete /tmp staged H2 copies after install (item 7 cleanup).
47. Re-verify R4 demo output quality on the real run (item 9).
48. asciinema cast trim if the recording captures dead time.
49. Beat cadence honesty: supervision beats are per-turn; wall-clock gaps
are covered by the detached stack — document that in the next report's
machine-state line so future sessions don't re-derive it.
50. Keep the doom-loop arithmetic on every report's front page until the
union fix lands: 3 reboots in ~17 h vs 12.9 h gate requirement.

## g) THREE QUESTIONS I CANNOT ANSWER MYSELF

1. **§g1 — authorize the skip-union implementation?** Multi-run resume
   (`--resume` accepting a run-set/union) turns ANY mid-run death into a
   ~30k ≈ 2.3 h re-scan instead of up to 12.9 h, making the census
   reboot-proof at the scan-CLI + scanstore semantics level. It is exactly
   the change I will NOT make without your go (flagged in two reports
   already). If yes, I implement it post-B with the read-model round-trip
   tests green before any wave-C launch.
2. **§g2/§g3 — auto-chain on B's gate PASS?** B exits ~20:33 CET. Without
   an answer I will NOT launch wave C: I will verify gate + snapshot +
   restore-verify, record everything, and HOLD single-flight for your
   instruction. If you pre-authorize the chain, say how far (C only? C +
   poisonfix? C + zone-ch batch 1?).
3. **§g3-R4 — public README embed without review?** The demo GIF renders
   post-B into assets/ and its natural landing is the public README
   (quickstart section). Embedding is a public-facing change — do you want
   to eyeball the GIF first, or do I push the embed straight after a
   self-review?

---

Supervision continues until B exits (~20:33). Then waiting for instructions
per this report's §g.
