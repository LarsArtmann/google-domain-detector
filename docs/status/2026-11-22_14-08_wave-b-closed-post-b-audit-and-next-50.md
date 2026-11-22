# Wave B closed + post-B bundle: comprehensive audit, next-50, and questions (2026-11-22 14-08)

Resumed from the 2026-11-15 14-37 handoff (report-then-WAIT directive).
Found wave B attempt 5 COMPLETE between sessions; executed the full
post-B bundle. This report is the a-g status the owner asked for,
including the self-audit (what was forgotten, what could be better).

## a) FULLY DONE (this session, all verified)

1. **CI on the handoff commit `c95507c` confirmed GREEN** (push run +
   scheduled Soak runs) — the handoff's step 2.
2. **Wave B exit evidence verified line-by-line**: wrapper logged
   `pass-1 gate PASS — wave tranco1m-b done (no cleanup needed)`
   20:05:04; stderr summary `171573 domains: workspace certain 28,020 /
   likely 7,818, gcp 2,269, errors 7,063 (nxdomain 4,604, timeout
   1,970, servfail 489)`; run-id file = `01M7EZJSEJKXNSPNEQ4401AE3S` =
   the journaled run; workload arithmetic closes exactly (171,573
   scanned + 11 unparseable inputs + 228,416 resume-skipped = 400,000).
3. **Snapshot restore-verified on a COPY** (production db untouched):
   sha256 sidecar OK (repo-root-relative path gotcha handled),
   PRAGMA integrity_check ok, 866,931 journal events in
   meta_stream_log + 503,542 projection rows, and `replay --run
   01M7EZJSEJKXNSPNEQ4401AE3S` from the COPY reproduced the live
   stderr summary byte-identically — the journal-alone report is
   proven.
4. **census.md wave ledger row** flipped to
   `**done 2026-11-15 20:05 (attempt 5, pass-1 gate PASS first try)**`
   with the full completion narrative; **TODO_LIST T12–T13 row**
   updated (B DONE, C gated on owner GO); both swept into `55715e0`.
5. **docs-gates repaired to green**: the last unharvested audit-all
   item (2026-11-11 report's "Census journal snapshot routine") got the
   honest done-at annotation + `fresh:skip` marker (its tokens are all
   DF-generic — unharvestable by construction). All four gates green.
6. **H2 dead-wiring fixes INSTALLED and live-verified** (`45f1c7b`):
   census-wave.sh's unused `GDD_CLEANUP_BAR_PCT` variable deleted
   (census-cleanup.sh reads the env itself); census-gate.sh's jsonl
   branch dropped the never-consumed 7th `errors` field (read + printf
   now match the stderr branch's 6-field shape). Verified with
   shellcheck + bash -n + LIVE smokes: the gate re-run on wave B's real
   stderr reproduces the completion verdict (unknown 1.43% vs 2%
   budget, coverage PASS), and a synthetic jsonl parses per-kind counts
   exactly. The /tmp-staged copies from the handoff were eaten by the
   tmp cleaner — re-derived from the documented spec, same fixes.
7. **R4 demo shipped** (`a987774`): scripts/demo-record.sh now runs
   end-to-end. Two real defects found and fixed on its first-ever run:
   (a) `asciinema rec <<heredoc` hangs forever on non-tty parents (the
   heredoc feeds asciinema's stdin relay, not the child shell) —
   reworked to a file-fed session via `--command "bash --noprofile
   --norc …"`; (b) vhs `Set Shell` accepts NO arguments (parser:
   "Invalid command: -"). Polished tape: cd into the work dir +
   relative commands (no /tmp paths on screen), self-contained
   `demo-gif.db` (GIF shows exactly 1 run), 1600x900, ~35 s, 855
   frames. Eyeballed four frames incl. the tail: live verdict-colored
   scan, runs table, show table, replay section all render correctly.
   assets/demo.gif committed (1.1 MB).
8. **R2 rank-band recalibration** (census.md + REPORT.md): error
   classes split by RANK from the final pass-1 jsonl (input order):
   unknowns are FLAT across the four 50k bands (1.53 / 1.48 / 1.26 /
   1.52%) — the mid-pass poller's declining trend was a forwarder-
   warmup transient, NOT rank structure; the global 2% D3 budget needs
   no per-band tightening. NXDOMAIN is strongly rank-dependent (0.65 →
   6.02%). Band totals reconcile with the gate exactly (ws 35,838, gcp
   2,269, nx 4,604, unknown 2,459).
9. **C6 baseline supersession audit** (read-only journal walk): exactly
   **25 poisoned** (c4 10 + c5 15, pilot/soak 0) and the findings'
   domain set is byte-identical (diff-verified) to
   `reports/census/waves/tranco1m-a-poisonfix.txt` — the T11 input is
   current and the closure command is staged.
10. **T15 report updated** (reports/census/REPORT.md, gitignored by
    design): run inventory completed with FULL ULIDs for all four
    wave-B attempts (recovered from meta_map), wave tallies flipped to
    CLOSED, rank-band distribution table + calibration read added.
11. **C8 closing report written, indexed, pushed** (`4598464`); status
    README gained the 2026-11-21_20-28 row.
12. **FEATURES.md refreshed** (`4f702a5`): wave B → complete with
    verified numbers; demo recorder → FULLY_FUNCTIONAL ops tooling;
    GIF embed → PLANNED (owner-gated); last-refreshed stamp updated.
13. **Every daemon bundle inspected** (9 auto-commits this session):
    all benign sweeps of my own in-flight edits; `afd0fd3` (the last
    uninspected one) verified = an earlier FEATURES.md state. Two
    junk-message bundles of purely-my-own files were message-amended
    per the mixed-commit policy (unpushed, byte-identical tree).
14. **CI green on every pushed commit** incl. the final HEAD `4f702a5`;
    both remotes (Forgejo origin + github mirror) synced — github had
    drifted 4 commits behind and was caught up.

## b) PARTIALLY DONE

1. **T15 census report** — skeleton + wave A/B sections + wave-B
   distribution table exist; the FINAL whole-census distribution needs
   wave C + poisonfix closed.
2. **C6 closure proof** — baseline (25 poisoned) is proven and the
   input is byte-verified; the "= 0 poisoned" proof requires poisonfix
   to RUN first.
3. **R4 demo** — recorder + GIF fully done and verified; the README
   embed (the actual user-visible outcome) is held on §g3.
4. **T12–T13** — B done; wave C untouched (owner-gated).
5. **docs-gates freshness for THIS report** — the §f list below routes
   new items into TODO_LIST in the same push (the 14-37 precedent);
   until that lands, this report is not yet gate-clean.
6. **Host clock sanity** — observed date jumps across turn gaps
   (Nov 17 → 21 → 22) and metadata.yaml writing future timestamps;
   noted, NOT diagnosed (out of session scope, flagged in §e/§f).

## c) NOT STARTED (correctly gated, zero drift)

1. Wave C launch (owner GO + forwarder restart ritual — unbound-tcp is
   DOWN since B's exit).
2. Poisonfix execution (owner GO; input ready).
3. RulesetV2 stamp (owner decision package ready; MUST precede wave C's
   binary build).
4. README GIF embed (§g3).
5. Multi-run skip-union (§g1) — design not started, only justified.
6. zone-ch batch 1 (prep done 2026-11-14; slot is post-C).
7. Nominet .uk send + MiTek verification (owner action).
8. GPG signing setup (owner, 8-ask #5).
9. BuildFlow#56 fleet fix (owner approval gate).
10. projectionhost settled-API issue FILING (draft ready; library work
    post-census).
11. The 14-37 §f operational backlog: PSI gate formalization in the
    wrapper, census-relaunch.sh encoder, NOTES.md backup ritual — all
    still open TODO rows, untouched this session (deliberately: they
    are bash-script changes and the census scripts were frozen until
    B's exit; after B's exit the session went to verification + demo).

## d) TOTALLY FUCKED UP (owned, with root causes)

1. **The demo-session generator edit fumble** — the worst of the
   session. I hand-edited a heredoc-quoting context I had not fully
   modeled: first edit landed a stray `end` line and mixed escaped/
   unescaped forms; then I mis-analyzed my OWN backslash algebra twice
   (once concluding garbage that was actually fine, once "fixing" into
   a design that needed ANOTHER rework). Cost: the demo pipeline ran 4
   times (~3 min each: go build + 60-domain scan + nix fetches + vhs
   render), two runs wasted. Root cause: incremental `multiedit` on a
   quoting puzzle instead of rewriting the whole generator block with
   `write` from a clean model. Lesson: quoting-context edits get
   whole-block rewrites, and the analysis happens BEFORE the tool call.
2. **Stale-read edit refusals, twice** — tried to `edit` census.md and
   the status README without a fresh in-session read after turn gaps;
   the tool refused (correctly). Cost: two wasted calls. This is a
   repeat of a handoff-documented failure class ("always fresh-view
   before edit after gaps") — I knew it and still did it.
3. **Gate smoke-test invocation error** — passed `--manifest` AFTER
   the file positional; the parser breaks at the first non-flag and
   read `--manifest` as the budget (awk coerced it to 0.00% → spurious
   FAIL). Diagnosed from the parser source within one step, re-ran
   wrapper-exact: PASS. But the initial confusion produced a false
   "the gate is broken" moment; a calmer operator reads the parser
   FIRST. Also: this failure mode is a REAL UX bug in the gate (flags
   after the file silently corrupt the budget) — item 1 of §f.
4. **Band-stat off-by-one** — first awk extraction used +15 for a
   14-char prefix; produced "nx=0 in every band" (impossible vs the
   summary). Self-caught because totals matched while kinds didn't.
   Cost: one wasted 61 MB scan of the jsonl. Lesson: derive substring
   offsets from the literal prefix length, not by eye.
5. **/tmp as a staging area, twice burned** — the handoff's staged H2
   fixes, the docs-gates log, and my built audit binary were all eaten
   by the tmp cleaner between sessions. I re-derived/rebuilt cleanly,
   but the handoff→resume chain lost its artifacts AGAIN (the same
   class the 14-37 report already flagged). The law should be: staged
   fixes live in the repo worktree (untracked or branch), never /tmp.
6. **Missed inspection window on `afd0fd3`** — the mixed-commit policy
   says inspect every daemon bundle; I noticed the commit in the log
   but only verified its content at report time (it's benign: an
   earlier FEATURES.md state). Nothing was at risk (CI green, content
   mine), but the verification was late, not eager.
7. **Minor**: the 14-37 handoff's /tmp forensics archives (61 MB jsonl
   etc.) are certainly tmp-cleaner-gone by now — I never checked or
   explicitly declared them disposable. No data loss (journal is
   truth), but the handoff artifact chain silently rotted.

## e) WHAT WE SHOULD IMPROVE (systemic, from this session)

1. **Gate arg-parsing robustness**: census-gate.sh `break` fires on the
   first non-flag, so flags placed after the file positional are read
   as positionals (`--manifest` becomes the budget → 0.00%). Fix: keep
   parsing flags until `--`, or error loudly on a post-file flag.
   The wrapper's invocation order is the only safe one today.
2. **Staging law**: patches staged for later install go in the repo
   (untracked file or a branch), never /tmp — the tmp cleaner has now
   eaten staged work in two consecutive sessions.
3. **Handoff artifact decay**: every handoff that references /tmp
   should either copy artifacts into `reports/` (gitignored side) or
   declare them disposable in writing.
4. **Demo tape determinism**: `Sleep 25s` assumes the scan completes
   in 25 s; the correct pattern is poll-for-completion (grep the
   progress footer) so GIF timing survives resolver slowness.
5. **GIF churn policy**: each re-record adds ~1.1 MB to git history;
   re-record only on meaningful CLI changes, consider squashing
   assets/ history at release tags.
6. **Clock hygiene**: metadata timestamps writing future values + date
   jumps across sessions → host NTP/timesyncd deserves a check before
   the next provenance-sensitive run (binary stamps, snapshot names).
7. **Manifest hygiene decision**: wave B's manifest had 11 unparseable
   entries (`_wildcard_.net.ph` et al.) — permanently unscanned, gate-
   accepted. Decide: sanitize the manifest for future waves (provenance
   sha changes) vs keep byte-immutable lists and carry the hole.
8. **Snapshot ritual is proven twice now** (20261113 + 20261115 both
   restore-verified) — promote it from report prose to a named step in
   census.md's exit checklist so it never depends on operator memory.
9. **R2 knowledge should tune the poller**: steady-state unknown is
   ~1.3–1.5%; GDD_WARN_PCT=2 fires only on genuine degradation —
   document that expectation in the wrapper (no code change needed).
10. **Quoting-context edits**: whole-block `write` rewrites beat
    incremental multiedit when backslash/heredoc escaping is involved.

## f) NEXT 50 (ordered: census chain first, then tooling, then fleet)

**Census chain (1–12):**
1. Owner: Wave C GO/NO-GO + chain composition (C only / C + poisonfix /
   C + zone-ch batch 1).
2. Owner: RulesetV2 stamp GO — MUST land before wave C's binary build.
3. Restart forwarder ritual + dig +tcp probe (5354) before any launch.
4. Launch wave C via census-wave.sh (explicit resume discipline:
   fresh wave, no `--resume latest`), PSI gate first.
5. Wave C exit: gate + snapshot + restore-verify on a copy (ritual).
6. Owner: T11 poisonfix-25 GO (input byte-verified; command staged).
7. Poisonfix execution + pilot-gate on the 25-domain manifest.
8. Poisonfix closure proof: `replay --audit-excluded <4 IDs>` → 0.
9. T15 final whole-census distribution section (after C + poisonfix).
10. R2 follow-up: compute wave-B TLD breakdown (.de/.ch/.uk/.co …)
    from the rank-ordered jsonl for the final report.
11. R2 follow-up: M365-class count from wave-B evidence for the final
    report (fingerprints are evidence-only today).
12. Whole-census closing report (C8 equivalent for the full census).

**Owner asks (13–19):**
13. Owner: §g1 multi-run skip-union authorization.
14. Prework (if §g1 likely): skip-union design note (journal-side union
    of N runs' observed streams; resume CLI shape).
15. Owner: README GIF embed decision (§g3).
16. Owner: Nominet .uk email send + MiTek ID verification.
17. Owner: GPG signing setup decision (8-ask #5).
18. Owner: BuildFlow#56 fleet fix approval (pre-commit hook rebuild).
19. Owner: M365 surface decision (post-census per the 02-18 package).

**Zone/TLD work (20–23):**
20. Verify the crt.sh daily timer's in-repo landing (the 04:37 fire —
    provenance fix was pending "next 04:37"; check it stuck).
21. zone-ch batch 1 launch (post-C slot; prep done, re-verify manifest
    sha before launch).
22. zone-ch batches 2–6 scheduling (~8 days total).
23. .uk scan setup once Files.com credentials arrive.

**Tooling/code (24–33):**
24. census-gate.sh: harden arg parsing (flags-after-positional hazard
    found live this session) + a pinning test.
25. PSI launch gate formalization in census-wave.sh (14-37 §e2 row).
26. scripts/census-relaunch.sh ritual encoder (14-37 §e3 row) —
    encodes the env prefix so the 07:48-class drop cannot recur.
27. NOTES.md backup ritual (14-37 §e5 row; reports/ is gitignored).
28. Demo tape: replace `Sleep 25s` with a poll-for-scan-completion.
29. tmp-staging law: document in AGENTS.md gotchas (staged fixes live
    in the worktree, never /tmp).
30. Host clock/NTP check (metadata future-timestamps + date jumps).
31. Manifest hygiene: decide the 11-unparseable-entries policy (§e7).
32. Snapshot restore-verify promoted to a named census.md exit step.
33. Wrapper poller note: steady-state unknown ~1.5% (R2) documented at
    the GDD_WARN_PCT site.

**Library/ecosystem (34–36):**
34. File the projectionhost settled-API issue (draft ready; owner-
    gated filing) — post-census library work.
35. Next go-cqrs-lite bump: foldByEvent last-fold-wins re-verify
    obligation (AGENTS.md pinned; post-census).
36. go mod tidy / toolchain floor re-check on the next Go change
    (floor stays 1.27.2; BuildFlow#56 interacts).

**Docs hygiene (37–42):**
37. TODO_LIST harvest of THIS report's §f (same push; freshness gate).
38. Status-README archive pass: reports whose items are all resolved
    (2026-11-10/11-11 era) move to archived/ (cadence threshold 15 is
    near: 14 unarchived now).
39. DOMAIN_LANGUAGE.md: add "unparseable input" as a distinct term
    (never an unknown, never a domain verdict).
40. ROADMAP.md refresh: wave-B-complete reality, C/zone-ch/.uk slots.
41. census.md: link the R2 calibration block from the calibration
    ladder section (one pointer line, no duplication).
42. AGENTS.md: 2-line addition — census single-flight law now includes
    "demo recorder counts as a scan" (it has its own guard, but the
    law should name it).

**Verification debt (43–46):**
43. Full `buildflow` run on the next session that touches Go (this
    session was bash/docs only; CI covered the Go suite on every push).
44. `go test -race ./...` on the next code-change session.
45. Demo GIF tail frames verified post-final-render (done ad hoc this
    session at frame 840) — note the check into the script's comments.
46.crm tier CSV refresh from final wave data (downstream consumer,
    AGENTS.md pointer; after wave C).

**Bigger swings (47–50):**
47. Skip-union implementation (post-design, post-§g1).
48. Wave C rank-band live poller: per-band unknown display (R2 says
    flat — likely NOT worth it; decide with data in hand).
49. Census provenance page: PROVENANCE.md pattern extended to every
    wave's binary stamp (B has it; C should inherit the ritual).
50. Post-census sales handoff package: tier CSVs + TLD breakdowns +
    workspace/GCP/M365 classes as the CRM consumer's final input.

## g) THREE QUESTIONS I CANNOT ANSWER MYSELF

1. **Wave C chain composition**: with B closed and single-flight free,
   do you want C ONLY, C + poisonfix-25 (my recommendation — it is
   25 domains, minutes, and unblocks the closure proof), or
   C + poisonfix + zone-ch batch 1 queued behind it?
2. **RulesetV2 stamp timing**: the 02-18 decision package recommends
   stamping before wave C's binary build. Confirm GO (the stamp
   changes the ruleset provenance every C event carries) or defer
   with C running under `v1-2026-11-10`.
3. **README GIF embed**: assets/demo.gif is committed and verified.
   Embed it after the quickstart block (`![](assets/demo.gif)`), or
   keep the README text-only for now (1.1 MB payload on every page
   view of the GitHub README)?
