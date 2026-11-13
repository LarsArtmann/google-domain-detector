# Polish lane resumed: hostile-TXT journal poison root-caused, wave B relaunched, docs corpus reconciled

**Session:** 2026-11-13 08:55 → 17:20 CET (continuation of the 08-32
polish lane after the owner's "execute the whole list" directive).

## a) DONE

1. **crtsh-daily.sh wrong-root bug fixed and branch-tested** — path
   corrected (`$here/../..` → `$here/..`), plausibility guard added
   (.de ≥100k rows, `GDD_MIN_ROWS` override; all three guard branches
   verified via the crtsh fixture), suspect artifacts relocated from
   `/home/lars/projects/reports/census/zone-data/` into the repo,
   monitor.log merged, stray dir trashed. CRITICAL CORRECTION: the
   08-32 report's "delete /home/lars/projects/reports" step was WRONG —
   that tree is a real sibling git project; only its stray `census/`
   dir was crtsh fallout. AGENTS.md gotcha added (test the LITERAL
   path string).
2. **Owed gates closed** — `nix build` green post-go1.27.4 pin (no hash
   mismatch); `buildflow format --check` 20/0 (fingerprint drift
   resolved); pushes to both remotes watched, CI green at HEAD.
3. **Wave-runner anti-wedge** — external `scripts/census-watchdog.sh`
   (stalled jsonl + no completion line → WEDGED=1 / DEAD=2 alarm; all
   three verdicts branch-tested) + poller HEARTBEAT STALL inside
   census-wave.sh (folded in via new-inode install; verdicts
   functionally tested). TODO row closed.
4. **.ch census input built** — ch.txt (1.44 GB AXFR) reduced to
   2,660,632 unique registrable apexes (zone-ch.txt, 43 MB, sha256 in
   MANIFEST.sha256; 0 non-apex lines, 0 uppercase). Pre-filter row
   closed as superseded by the existence oracle.
5. **README polish + consumer sweep** — Homebrew tap install added
   (verified formula), enrich round trip + --max-runtime documented;
   GitHub code search = 32 hits all self-repo → zero external Go
   consumers for v0.1.0 (row closed).
6. **Hostile-TXT journal poison — root cause → fix → wedge recovery**
   (the session's core work, §c below). TODO rows closed: T16 delta,
   interrupted-summary, resume-skip accounting, CSV enrichment,
   max-runtime, lint-zero, fuzzing, cli-coverage 90.7%, IPv6, resolver
   guidance, retract dry run, CAA→ROADMAP, version-freshness guard,
   cmdguard-lint CI, lint/vuln CI, tidy, vendorHash, crtsh
   scheduling, pre-filter, memory row, watchdog, .uk draft-ready,
   consumer sweep, UA note, buildflow format, vendorHash re-pin.
7. **docs-health pass (audit upkeep)** — audit-all's 37 unharvested
   items → 60 inline annotations across 12 status reports (done-at
   hashes / superseded-by-the-oracle / won't-implement with owning
   lane / routed to TODO+ROADMAP rows); 28 fully-resolved reports
   archived with banner + bulk manifest; **real bug found and fixed**:
   firstSystemNameserver never checked scanner.Err() (`41777d0`).
   One order-flaky audit test fixed (attribution instead of
   input-order assumption, `bb46ddb`). All five docs-gates green.
8. **AGENTS.md diet** — 349 → 222 lines; G4 numbers + negative-cache
   lesson moved to docs/census.md §"G4 journal capacity"; every
   distinct fact preserved in condensed form.
9. **T15 census report written** — reports/census/REPORT.md: 17-run
   inventory, excluded runs pinned with full IDs + reasons, mechanical
   supersession accounting, hostile-TXT incident, wave tallies.

## b) PARTIALLY DONE

1. **Wave B (T12–T13)** — attempt 2 running since 16:53: explicit
   `--resume 01M79XW7SF918E61NYVM000001`, expect-skip 96,215
   (attempt 1 journaled 96,215 domains before/despite the wedge),
   ~304k domains remaining, gdd-20261113b, w4/10s via 5354, watchdog
   armed. ETA ~24–30 h; then gate → T14 cleanup → wave C → poisonfix-25.
2. **Audit upkeep** — citation verification green; remaining: re-shoot
   the libs index render proof + re-audit go-cqrs-lite verdicts (row
   updated to the narrow remainder).

## c) The hostile-TXT incident (root cause → fix → recovery)

- **11:52:49** wave B attempt 1 rendered `bide-et-musique.com` — its
  google-site-verification TXT carried raw invalid-UTF-8 bytes;
  jsontext (JSON v2) REFUSED the stdout line and the render path died
  (jsonl frozen 4.5 h). The journal event (seq 190202) was already
  written — with invalid UTF-8 inside a CBOR text string — so every
  journal fold reaching it failed (`corruption:event.decode_payload_failed`).
  Journal writes themselves stalled ~16:13 (SQLITE_BUSY class).
- **Quantified** via a hand-rolled CBOR walker over all 245,178
  journal events: exactly ONE poison event.
- **Fixes (all fixture-pinned to the real payload):** scan-boundary
  UTF-8 coercion (`classify.SanitizeEvidence` + `scan.sanitizeResult`,
  evidence copied — the first version mutated the shared slice and the
  race detector caught it), encode-side payload sanitize
  (`scanstore.sanitizeObservedPayload`), decode-side CBOR repair
  (`scanstore.cborsanitize.go` — journal stays append-only), external
  watchdog + heartbeat. Lint driven back to 0 issues; full -race suite
  green.
- **Recovery:** the NEW watchdog alarmed 16:22 (its first live catch —
  jsonl was already 4 h stale); SIGINT drained cleanly at 16:46 (run id
  recorded, SIGINT-drain contract held); the wrapper's retry path
  resumed attempt 2 on the fixed binary with expect-skip computed from
  the journal. First-run stdout loss (41k lines) is render-only — the
  journal holds all 96,215 observations.
- **Supersession audit finally ran** (it was blocked by the poison):
  pilot 0, soak3 0, c4 10, c5 15 = **25 poisoned domains** — the
  hand-derived 948-domain poisonfix premise was FALSIFIED by the
  mechanical audit; the correction list was regenerated from audit
  output.

## d) Mistakes not to repeat

1. The 08-32 report's `rm -rf /home/lars/projects/reports` step —
   verified before executing would have been mandatory even without
   the safety rules; the alien dir was three levels deep inside a real
   sibling project.
2. I pointed the `edit` tool at census-wave.sh while wave B ran on it —
   in-place rewrite, same inode, pending-offset corruption risk
   (documented AGENTS.md gotcha sharpened with the recovery recipe:
   revert byte-identical → build in /tmp → install).
3. `sanitizeObservedPayload`'s first version mutated the shared
   Evidence slice — caught by -race in the cli suite, fixed by copying.
   The journal poison fix and a data race in the same day both trace to
   "sanitize in place" thinking.
4. docs-gates' verdict vocabulary is narrower than the skill prose
   suggests (`done at` REQUIRES a backticked hash; "routed" is not a
   strike verdict) — five battery iterations to converge; use the
   gate's own marker regex as the spec.

## e) Machine state at report time

- Wave B attempt 2: RUNNING (pid family intact, watchdog pid 2338028),
  ~8k fresh observations in the resumed pass, unknown rate normal.
- HEAD `bb46ddb` pushed to both remotes, CI green (the docs-health and
  test-fix commits each got their own green run; the 15:11 failure was
  the sibling's sweep, superseded).
- docs-gates: 5/5 PASS. golangci-lint: 0 issues. -race suite: green.
- AGENTS.md 222 lines; TODO_LIST 17 open rows (post-B: T14, poisonfix-25,
  wave C; owner: www-strip policy, GPG; long-tail: cqrs-lint triage,
  runs --format, file-size debt, io-ratchet, contention post-B,
  FormatsForShape, TXT truncation, M365 fingerprints, enrichment,
  dedup; libs render proof).

## f) Next tasks

1. Wave B gate on completion (expect-skip 96,215 reconciles); T14
   cleanup loop arms automatically on gate FAIL.
2. Wave C launch after B's gate PASS (single-flight frees).
3. poisonfix-25 correction pass (input regenerated from the audit);
   closure = audit reporting 0.
4. Tomorrow 04:37 timer run = the crtsh wrapper's first legit
   verification (plausibility guard active).
5. T15 final distribution section after C + poisonfix close.
6. Nominet: owner sends the drafted email (docs/zone-data/nominet-uk.md).

## g) Owner questions

1. **g1 (resolved by events):** the wedged nxfix runner question is
   moot — wave B's own wedge recovery exercised the manual-gate path;
   both waves' artifacts are reconciled and snapshotted.
2. **g2:** wave B attempt 2 is running on the TCP path (w4/10s, ~8
   obs/s early rate). Wave C chains automatically per the standing
   decision; say the word to hold.
3. **g3:** Nominet draft is ready for your review/send — no further
   action possible without your identity details (MiTek verification).
