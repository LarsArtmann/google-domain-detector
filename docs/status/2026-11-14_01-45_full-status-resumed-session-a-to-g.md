# Full status (a–g): resumed session — reconciliation, staleness discovery, code rows, wave-B watch

**Session:** 2026-11-13 21:15 → 2026-11-14 01:45 CET (after the owner's
execute directive; the 21-15 resume report covers its first half, the
01-38 code-window report covers the middle; this is the comprehensive
a–g closeout of the whole arc). **HEAD at report time: `1e28325` + the
rename commit; both remotes synced.** Sibling session live throughout
(replay/audit lane, then the file-size split).

## a) FULLY DONE

1. **Git reconciliation of concurrent-session landings** — reviewed
   every daemon-swept commit line-by-line before pushing (go-codec
   promotion, F005/F007 anchor move, AGENTS diet, cqrs-lint triage);
   verified build/vet/tidy-zero-diff/scanstore tests; pushed both
   remotes; CI watched green on my pushes.
2. **`runs --format` with FULL run IDs** — the poisonfix-debt row closed:
   registry-pinned full enum; data formats carry full IDs + raw fields,
   human formats keep short IDs; 4 new tests.
3. **Read-model staleness race root-caused and fixed** — folds live in
   projection workers, not dispatch (experiment-proven); `Open` +
   immediate query deterministically saw an EMPTY ledger on an unfoldled
   journal; `scanstore.WaitProjectionsSettled` (bounded, exhaustive,
   static-error style) wired into runs/show/enrich; regression test
   pins the unfoldled-journal round trip; `--resume` proven immune
   (journal walk); AGENTS.md gotcha + library TODO row recorded.
4. **Data-purity fix** — run-count footer leaked into csv/tsv; gated to
   table/markdown in both the runs table and the supersession audit
   table; purity tests pin both.
5. **M365 fingerprints** — `mx_microsoft`/`spf_microsoft` evidence-only,
   verdict invariance pinned (Microsoft never upgrades Google verdicts),
   SPF parsers deduplicated, Counts untouched (journal schema v1
   mid-census), FEATURES.md updated.
6. **FormatsForShape (E27) decided-closed** as superseded by the T22
   registry pin (explicit product contract beats auto-generation that
   would include bystander formats).
7. **TXT truncation row struck** — was closed-with-evidence since the
   00-03 session (cloudflare.com 2,539-byte TXT parses cleanly; class =
   resolver distress), strike just missing.
8. **Describe-battery eyeball** — struck with evidence (all commands and
   flags current).
9. **TODO_LIST + AGENTS.md truth pass** — strikes for every row this
   window completed, staleness law recorded, library request row added;
   all five docs-gates green after each docs pass.
10. **Status reports written + indexed + pushed** (21-15, 01-38, this
    one) — including fixing my own 02-15 filename lie (estimated time
    instead of `date`; renamed to the verified 01-38).

## b) PARTIALLY DONE

1. **Wave B attempt 2** — 115,791 scanned at the 01:23 poll (~3.7/s
   sustained, ~38% of ~304k), jsonl at 119k+ lines, runner alive 8h+,
   **0 watchdog alarms**. Poller cadence exact (30-min lines; my earlier
   "lagging" read was my own clock error). ETA ~15:00-16:00 today;
   wrapper auto-gates with `--expect-skip 96215`.
2. **Sibling session's lanes in flight** — `--audit-excluded` wave names
   (replay.go mid-refactor: the branch's CI red is exactly their 2 WIP
   lint findings) and the file-size split (cli_test.go → focused
   cli_*_test.go files appearing, plus ranges/render/scan test splits
   staged). Both land under their hand; branch greens when they finish.
3. **Census chain** — B → gate → wave C → poisonfix-25 → T15-final →
   closing report: only B is running; everything else correctly sequenced
   behind the single-flight law.

## c) NOT STARTED (blocked or deliberately sequenced)

Post-B rows (T14 loop, wave C launch, poisonfix-25 correction, closure
audit, T15 final distribution, rank-band calibration, TCP contention
measurement, zone-ch scan — all single-flight-gated); owner rows
(www-strip policy, GPG signing, Nominet email); asciinema demo; audit
upkeep remainders (libs index render proof, go-cqrs-lite verdict
re-audit); enrichment join / dedup-by-registrant (post-census ideas).

## d) TOTALLY FUCKED UP

**Nothing shipped broken; four owned process misses:**

1. **Report filename written from an estimated clock** (02-15 vs actual
   01:38) — a `date` call is the ritual's first step and I skipped it
   when writing that report; caught and fixed by rename within minutes,
   but the commit churn was avoidable.
2. **`buildflow format` ran repo-wide** and reformatted the sibling's
   in-flight files alongside mine — content-neutral but I should have
   scoped formatting to my files only in a live-shared tree.
3. **The throwaway experiment file sat in the repo ~40 min** and broke a
   `-race` suite run (its loop pins the pre-fix race by design) before I
   deleted it — throwaway experiments belong in /tmp or get deleted
   immediately after their conclusion is recorded.
4. **A sloppy import-block edit** mangled runs.go mid-flight (caught by
   build in two edits, before any commit) — edit-sediment from trying to
   fix lint findings with targeted string matches instead of one clean
   rewrite.

## e) WHAT WE SHOULD IMPROVE

1. **`date` before every artifact named after time** — the one hard rule
   I broke; filenames are forever, estimates are not timestamps.
2. **Lane fences between live sessions** — worked this time (I backed
   off replay/audit on sight; the sibling extended my audit test instead
   of colliding), but it depended on reading their diffs char-by-char;
   a one-line lane claim per session would make it mechanical.
3. **Scoped formatters in shared trees** — `buildflow format` is
   repo-wide by design; in a live-shared tree, run formatters per-file.
4. **CI red mid-window under concurrent sessions** — my pushes improved
   the finding count each time, but a "their-lane-red" note in each
   report (done here) should become the standing pattern so red CI is
   attributable at a glance.
5. **The projection-staleness fix is interim by design** — the library
   (`projectionhost`) should own a settled-API; the repo-local wait
   duplicates logic every future go-cqrs-lite consumer would need.
6. **Edit-tool hygiene under lint-fix churn** — prefer one `write` of a
   small function over three escalating string matches (miss #4).

## f) UP TO 50 THINGS TO GET DONE NEXT

**Census critical path (single-flight, in order):**

1. Wave B completion ~15:00-16:00 → automatic gate PASS check
   (`--expect-skip 96215`).
2. On gate FAIL: T14 cleanup loop + `census-gate.sh --manifest` reconcile.
3. Launch wave C (`tranco1m-c`, fresh pass, gdd-20261113b, w4/10s, 5354,
   TCP preflight) when B's single-flight frees.
4. Attach a fresh watchdog to wave C.
5. Verify at least one auto-snapshot landed in reports/census/snapshots/
   post-B.
6. Poisonfix-25 correction pass (`tranco1m-a-poisonfix.txt`) after C.
7. Closure proof: `replay --audit-excluded <4 run IDs>` reports 0.
8. T15 final whole-census distribution (pin the replay command).
9. Closing status report + index + push.
10. Verify crt.sh timer run (~04:37-05:00) lands in-repo with the
    plausibility guard accepting ≥100k .de rows.
11. Watch CI on each pushed HEAD through the chain (release law).

**Code work (unblocked; mind sibling lanes):**
12. Branch green when the sibling's wave-names + file-split land —
then verify `--audit-excluded wave:NAME` end-to-end on the real
journal.
13. Review the sibling's file-split for the 350-line gate (9 files →
focused files; their split-in-progress already shows the pattern).
14. Library: go-cqrs-lite `projectionhost` settled-API (post-census
window; this repo's `WaitProjectionsSettled` is the reference).
15. io-history-ratchet `--profile-io` investigation (56 files vs median 10).
16. `incoming/outgoing-num-tcp` contention measurement post-B → NOTES.md.
17. Poller rank-band unknown-rate expectations calibrated on wave B's
pass-1 (~2.8% band vs A's 1.53%) + bound the "none" share per band.
18. Audit upkeep: docs/review/libs index render proof.
19. Audit upkeep: go-cqrs-lite/cqrs-htmx verdict re-audit (event-store
work may have flipped premises).
20. STACK.md row for go-codec as a declared direct dependency.
21. CHANGELOG Unreleased coherence check before the next tag.
22. Full `-race` suite once on the settled HEAD (one lane at a time).
23. asciinema demo / GIF for the README (E20).
24. `pgrep -f 'gdd[^ ]* scan'` = exactly ONE scan through C + poisonfix
(single-flight law enforcement between waves).

**Zone-data track:**
25. Scan `zone-ch.txt` (2,660,632 apexes) when Tranco frees the
single-flight.
26. Nominet .uk email — OWNER (draft ready).
27. .de daily crt.sh fallback continues (row 10 verifies the timer).

**Owner decisions:**
28. www-strip policy (26 wave-A inputs like www.edu.cn).
29. GPG signing policy for agent commits.
30. Attended vs auto-chained wave-C launch (open since 21-15 §g).

**Post-census ideas (parked):**
31. Enrichment join (imprint/contact crawl or master-data cross-ref).
32. Dedup by registrant/organization (needs zone/WHOIS data).
33. Kith CRM delta rescans: retire the CRM diff logic in favor of
`replay`-based deltas now that T16 exists (confirm CRM side).

**Hygiene / process:**
34. Another docs-health HARVEST cycle over TODO_LIST (90+ [x] rows).
35. Quiet/fix the stale LSP golangci diagnostics (IDE noise vs 0-issue
truth).
36. Keep census-wave.sh's inode-safety law documented for every future
mid-run edit (new-inode installs only).
~~37. Consider a session-start lane-claim convention for concurrent~~ **Won't implement — declined — unproven process idea; the report-header lane/red-attribution notes (§e4) are the adopted pattern.**
agent sessions (see §e2).
38. Docs: document jsonl-growth as the primary wave-liveness signal,
poller lines as secondary (see §e4 / 01-38 §e6).
39. After the sibling's split lands: re-check `golangci-lint run` = 0
across all packages (lint gates travel with moved code).
40. Keep CHANGELOG current for the runs-format + staleness-fix features
(user-facing surface changed this window).

## g) QUESTIONS I CANNOT FIGURE OUT MYSELF

1. **Overnight autonomy for the census chain:** wave B gates ~15:00-16:00
   today; do you want me attending the B→C→poisonfix handoffs manually,
   or pre-stage an auto-chain that launches C on a B-gate PASS (accepting
   unattended failure handling)? (Asked since 21-15; defaulting attended.)
2. **www-strip policy** (26 wave-A inputs like `www.edu.cn`): skip as
   malformed, or normalize to the bare public suffix and scan?
3. **Sibling-session cadence:** the replay/audit + file-split lanes are
   theirs and in flight — should I take the NEXT unblocked rows
   (asciinema, audit-upkeep, STACK.md/CHANGELOG hygiene) while they
   finish, or hold off code-adjacent lanes entirely until their session
   ends to keep the tree single-writer?
