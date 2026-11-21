# Wave B complete + post-B bundle — closing report (2026-11-21 20-28)

Supervision resumed from the 2026-11-15 14-37 handoff. Wave B attempt 5
had completed unattended between sessions: the wrapper auto-sequenced
gate + snapshot exactly as designed, the watchdog exited clean, and
every verification this session ran came back green. This report closes
the B chapter and records the post-B bundle.

## a. Wave B completion (verified end-to-end)

- **Attempt 5 completed 2026-11-15 20:05** after 12.2 h (07:50→20:05):
  zero reboots, zero watchdog alarms, PSI quiet throughout,
  single-flight held.
- **Pass-1 gate PASS on the FIRST try** (no cleanup loop): 171,573
  scanned + 11 unparseable + 228,416 resume-skipped = 400,000 manifest
  exactly. Summary `workspace certain 28,020 / likely 7,818, gcp 2,269,
  errors 7,063 (nxdomain 4,604, timeout 1,970, servfail 489)`. Run
  `01M7EZJSEJKXNSPNEQ4401AE3S`. Unknown 2,459 = 1.43% < 2% budget.
- **Snapshot restore-verified on a COPY** (never the production db):
  sha256 sidecar OK, PRAGMA integrity_check ok, 866,931 journal events
  in meta_stream_log, and `replay --run 01M7EZJSEJKXNSPNEQ4401AE3S`
  reproduced the live stderr summary byte-identically from the journal
  alone.
- CI on the handoff commit `c95507c` was green (push run + two
  scheduled Soak runs); ledger row and TODO_LIST updated, pushed.

## b. Post-B bundle (done this session)

- **H2 dead-wiring fixes installed** (`45f1c7b`, new-inode-safe since
  no wrapper/watchdog lives): census-wave.sh's unused
  GDD_CLEANUP_BAR_PCT assignment deleted (census-cleanup.sh reads the
  env itself); census-gate.sh's jsonl branch read a seventh `errors`
  field nothing consumed — read + printf now match the stderr branch's
  six-field shape. Verified LIVE: the gate re-run on wave B's stderr
  reproduces the completion verdict, and a synthetic jsonl parses
  per-kind counts exactly. (The /tmp-staged copies from the handoff
  were lost to the tmp cleaner; re-derived from the documented spec
  and re-verified — same fixes, no drift.)
- **R2 rank-band recalibration** (census.md): unknowns are FLAT across
  the four 50k bands (1.53% / 1.48% / 1.26% / 1.52% — the mid-pass
  poller's declining trend was a forwarder-warmup transient, not rank
  structure); NXDOMAIN is strongly rank-dependent (0.65% → 6.02%).
  Wave C should expect much higher dead-domain rates at a steady ~1.5%
  unknown; the gate already excludes nx from the unknown budget.
- **R4 demo shipped** (`a987774`): scripts/demo-record.sh runs
  end-to-end; assets/demo.gif (1.1 MB, 1600x900, ~35 s) walks a live
  60-domain scan through runs → show → replay from a self-contained
  demo-gif.db. **README embed held on owner question §g3.**
- **T15 wave-B sections** (reports/census/REPORT.md, gitignored by
  design): run inventory completed (all four attempt run IDs, full
  ULIDs), wave tallies updated to CLOSED, rank-band distribution table
  + calibration read added.
- **C6 baseline supersession audit** (read-only journal walk): exactly
  **25 poisoned** (c4 10 + c5 15, pilot/soak 0) and the audit findings'
  domain set is byte-identical to
  `reports/census/waves/tranco1m-a-poisonfix.txt` — the T11 input is
  current. Closure proof (= 0 poisoned) lands after poisonfix runs.

## c. Failures owned (all self-caught, zero permanent damage)

1. **asciinema stdin-relay hang**: the demo script's original
   `asciinema rec <<heredoc` recorded one event and blocked forever —
   the heredoc feeds asciinema's stdin relay, not the child shell, and
   a non-tty parent never produces the keystrokes. Root-caused via the
   534-byte cast; reworked to a file-fed session (`--command "bash
   --noprofile --norc $work/demo-session.sh"`).
2. **vhs `Set Shell` takes no arguments**: `Set Shell bash
   --noprofile --norc` failed the tape parser ("Invalid command: -");
   the rc-noise fix moved to a leading `Type "clear"` + the session's
   own --noprofile --norc.
3. **Band-stat off-by-one**: my first awk extraction of `error_kind`
   used a +15 offset (prefix is 14 chars) — classified every error as
   unknown and showed nx=0 in all bands, which contradicted the
   summary; totals matched (35,838 ws / 2,269 gcp) which isolated the
   bug to the kind split. Fixed offset re-run reconciled exactly.
4. **Gate smoke flag-order mistake**: passing `--manifest` AFTER the
   file positional made the parser break at the file and read
   `--manifest` as the budget (awk coerced it to 0.00% → spurious
   FAIL). The wrapper passes flags first; re-run with wrapper-exact
   ordering produced the true PASS.
5. **/tmp volatility**: the tmp cleaner ate the staged H2 fixes, the
   docs-gates log, and the built audit binary between sessions —
   re-derived from spec, re-ran gates, rebuilt. Lesson amplified:
   anything that must survive a session gap lives in the repo, not
   /tmp.

## d. Machine state at close

- HEAD `a987774` on master, pushed to origin (Forgejo) + github mirror;
  CI queued on the push (checked below before closing); latest
  scheduled Soak run green.
- **Single-flight FREE** — no `gdd scan`, no wrapper, no watchdog.
- **Forwarder DOWN** (unbound-tcp not running since wave B's exit) —
  any wave C launch MUST redo the restart + dig-probe ritual.
- Tree clean; reports/ artifacts (REPORT.md, snapshots, wave dirs)
  local-only per .gitignore design.

## e. Owner-gated items (surfaced, NOT executed)

1. **Wave C GO/NO-GO** (14-37 §g2/§g3): C only, C + poisonfix-25, or
   C + zone-ch batch 1. Single-flight is free; launch mechanics in
   TODO_LIST.
2. **M365/RulesetV2 stamp MUST land before wave C's binary is built**
   (decision package: docs/review/2026-11-14_02-18-...). This is the
   hardest pre-C dependency.
3. **T11 poisonfix GO** — input re-verified this session; closure proof
   command staged (`replay --audit-excluded <4 run IDs>` → expect 0).
4. **README embed review** (14-37 §g3) — the GIF exists now, awaiting
   the embed decision.
5. **§g1 multi-run skip-union** — still the biggest resume-cost lever
   (would have cut the B resume from 171,584 to ~30k domains).
6. Standing 8-ask package (Nominet send, GPG signing, BuildFlow#56
   rebuild, M365 surface) unchanged from the 02-18 report.

## f. What was NOT done (honesty)

- Wave C not launched (owner-gated).
- Poisonfix not executed (owner-gated).
- No census.md/REPORT.md distribution FINAL section (needs wave C +
  poisonfix closed).
- The 14-37 report's §f operational backlog (PSI gate formalization as
  a wrapper check, census-relaunch.sh encoder, NOTES.md backup ritual)
  remains open in TODO_LIST — untouched this session.
