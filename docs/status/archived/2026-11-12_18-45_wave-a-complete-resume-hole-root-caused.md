> **Live remainder:** every item resolved as of the 2026-11-13 docs-health pass (docs-gates green); open census work lives in TODO_LIST.md + docs/census.md.

# Status — wave A complete; 2,731-domain resume hole root-caused and gated shut

**2026-11-12 18:45 CET · @ master 0bd3deb + this session's changes (commit
follows immediately; both remotes) · census lane (scripts/docs/TODO only —
internal/ untouched)**

Resumed the 04-15 waiting point. Wave A had finished at 04:51 CET
(census-gate: PASS, unknown 1.53% < 2%). Ran the completion protocol and
it uncovered a real coverage hole — root-caused to the journal, gated
shut, and documented. Wave B remains HELD for owner go.

## a) DONE (this session)

1. **Wave A completion protocol.** Gate verdict PASS from monitor.log;
   manual snapshot `reports/census/snapshots/tranco1m-a-complete.tar.gz`
   (54 MB tar + sha256; old-inode wrapper had no auto-snapshot); final
   accounting below; T11 struck in TODO_LIST (freshness pre-flight SAFE).
2. **Coverage hole found and root-caused.** run1.jsonl holds 97,265
   domains; the manifest holds 100,000 unique (verified: raw == unique ==
   100,000, no case/trailing-dot dupes). Full ledger (every term
   journal-verified):
   - 4 unparseable inputs (reported by the scan, correct)
   - **2,731 silently skipped by `--resume latest`** — it resolved to
     soak3 (run `01M74VTPD4FZFVY389W8000YTF`, aborted, 2,731
     domain_observed events with full evidence in the journal;
     LoadResumeSource skips exactly those), 2,727 of them literal
     manifest lines + 4 as www-stripped forms
   - 26 further inputs scanned under www-stripped names (`www.edu.cn` →
     `edu.cn` — the 04-15 §f3 artifacts, now diagnosed end-to-end)
   - 100,000 − 4 − 2,731 = 97,265 = the scan's own summary ✓
   - The health gate passed because coverage is not a health metric.
3. **Gate hardened (scripts/census-gate.sh).** New `--manifest FILE` +
   `--expect-skip N` coverage assertion: scanned + unparseable +
   expected-skips must reconcile with unique manifest lines; gap exits
   **4** (health FAIL stays 1 = cleanup remedy; coverage gap is a
   reconcile-don't-clean remedy). Verified against the real wave-A
   stderr: reproduces exactly "missing 2731", exit 4; `--expect-skip
   2731` → coverage PASS; legacy no-manifest mode byte-identical.
4. **Wrapper hardened (scripts/census-wave.sh).** Fresh out-dir = NO
   `--resume` (immune to soak/pilot journal poisoning); retries resume
   explicitly from `<out-dir>/run-id` (captured after every attempt —
   the "run <id> recorded" line prints even on abort); retry gate calls
   pass journal-derived `--expect-skip`; gate exit 4 skips cleanup and
   snapshots instead. Live-tested all three paths with a stub binary:
   fresh PASS / coverage-gap rc=4 no-cleanup / retry expect-skip=97,265
   reconcile PASS.
5. **90,834 root-caused (04-15 next-step 4).** No journal or manifest
   quantity reproduces it (per-run observations 7,204/1,725/113/3,153/
   3,090/7,243/4,999/97,265; distinct pre-run1 13,437; input overlap
   6,611 — none combine to it). First appearance: 14-44 report, fully
   formed, no derivation anywhere upstream. It was never a real number;
   the census plan's "63k already observed" estimate (plan T11 row) was
   likewise wrong (real single-source skip was 2,731, union overlap
   6,611). census.md wave ledger now carries the true numbers.
6. **Docs/TODO.** census.md: resume-hazard section + runner/gate bullets
   - ledger row. TODO_LIST: T11 struck, T11-patch + T12–T13 rows (held),
     gate-assertion row done, input-lane www-strip row + cli-lane
     resume-observability row (both sibling, fresh:skip-marked). 04-15
     report §b2/§f3 annotated RESOLVED inline. AGENTS.md gotcha added.
     docs/status/README.md re-indexed (Nov 11–12 rows were missing).
7. **CI/soak:** CI green on de9a968 and 7f5aa10; nightly Soak 03:17Z
   green.

## b) The finding, in one paragraph

`--resume latest` picks the most recently **started** run — an aborted
soak counts. It then silently skips every domain that run journaled;
the scanner prints nothing about skips, so a wave can pass its health
gate while missing 2.7% of its manifest (here: mostly high-rank domains,
since soaks process the manifest from the top). Wave A's 2,731 domains
are not lost: they sit in census.db under soak3's run id with full
evidence, same ruleset (mv1-2026-11-10) and resolver chain — coverage is
recoverable either by accepting soak3's observations or by a 2,731-domain
patch scan linked to wave A.

## c) OWNER DECISIONS (updated; supersedes nothing, adds one)

1. **NEW — wave-A hole:** accept soak3's 2,731 observations as wave-A
   coverage, or patch-scan them (ready command: retry-style pass with
   `--resume 01M755NSBKB09MXM0EB0001CZB` on tranco1m-a.txt into the
   wave-A out-dir — the hardened wrapper now does exactly this safely)?
   Patch cost ≈ 27 min at wave-A rates.
2. **STANDING — wave B launch:** still HELD for explicit go (04-15
   §g). The hardened wrapper is the one B should run (fresh pass, no
   resume, coverage-gated).
3. Still open from 04-15 §g: §g defaults confirmation, alert channel,
   archive ritual permission (cadence red at 20 unarchived vs 15).

## d) NOT DONE / HELD

- Wave B, T11-patch: owner go required.
- E17 census report: after wave C (pin excluded pilot `01M74VNNCJS6`).
- 7s-timeout + w2 soaks: single-flight window only (waves/patches done).
- Input-lane www-strip policy + cli resume-observability: sibling lanes,
  TODO rows filed.

<!-- fresh:skip sibling-scoped pending owner policy: input www-strip, cli resume accounting -->
