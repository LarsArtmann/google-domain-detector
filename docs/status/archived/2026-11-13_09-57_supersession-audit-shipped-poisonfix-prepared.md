> **Live remainder:** every item resolved as of the 2026-11-13 docs-health pass (docs-gates green); open census work lives in TODO_LIST.md + docs/census.md.

# Status Report — Supersession audit shipped, poisonfix pass prepared, wave-B warning investigated

**Written:** 2026-11-13 09:57 CET · **Session scope:** continuation of the
census lane under the owner's execute-until-done mandate — support work
only while wave B holds the single-flight (read-only journal work,
offline code+tests, docs; census-wave.sh and the running wrapper
untouched). Work happened on top of the daemon-swept tree; individual
commit shas are the daemon's, see git log.

## The one-line summary

The two supersession gaps the 08-57 self-review found are now CLOSED
TOOLING-WISE: the poison check is a shipped command
(`replay --audit-excluded`), the 948-domain correction-pass input list is
built and sha256-pinned in the ledger, and every §f item that could move
without the single-flight moved.

## a) FULLY DONE

1. **CI verification of the scanner-fix chain (release law)** — skipped by
   the 08-57 session (§d4): `gh run list` green on HEAD d2393bb AND on
   ae05f57 (wave-B launch); 7b45a1b (the fix) is an ancestor of both. The
   pushed range every future wave depends on is CI-green.
2. **Supersession audit as a product feature** (§f15/e2): new
   `scanstore.AuditSupersession` (single journal walk; per-domain
   last-wins provenance) + `replay --audit-excluded RUN1,RUN2` in the CLI
   (all `--format` renderers via one RenderTable path; findings exit 0
   with stderr counts; unknown run IDs FAIL the audit — a typo must not
   report clean; `--run/--since/--only` rejected with exit 2). Tests:
   3 scanstore unit tests (last-wins both directions, first-seen order,
   error taxonomy incl. ErrNoRuns on an empty journal) + 2 CLI e2e tests
   through the real scan path (poison/prove/stale legs + usage contract).
   `golangci-lint` 0 issues on both packages; `-race` green.
3. **Poisonfix input list built from the journal snapshot** (§f2): the
   three poisoned runs replayed from the nxfix-complete snapshot (never
   the live db): c4 `01M794AXA8YV…` (6 NX), c5 `01M794ZEB4HY…` (6 NX),
   pilot `01M74VNNCJS6…` (946 NX — exactly the 08-57 estimate). Union
   deduped = **948 domains** → `reports/census/waves/tranco1m-a-poisonfix.txt`
   (sha256 `ba24ae2ad210…`), ledger row in docs/census.md with derivation
   and the post-B launch recipe; TODO_LIST T11-poisonfix row tracks it
   (single-flight + owner go pending).
4. **Docs truth pass for the fix + audit**: CHANGELOG [Unreleased] (the
   user-facing false-NXDOMAIN fix §f13 + the audit feature), README
   (journal quickstart line + "Trusting a journal" section + the
   limitations bullet rewritten: NX verdicts are rcode-level truth since
   2026-11-13, cached-negative lies at the resolver remain possible),
   census.md (supersession-audit section, poisonfix ledger row, measured
   TCP-upstream sustained rate 4.0–4.6 dom/s §f12, throwaway-store rule
   §e5), TODO_LIST (audit done-row, poisonfix row, harvested §f16/32/33,
   the overstated nxdomain-bucket sentence corrected to manifest scope).
5. **CI/session ritual** (§f28): `gh run list --limit 5` at session start
   — all green incl. the nightly Soak.
6. **Wave-B 09:15 WARNING investigated** (§f6) — read-only: unknown rate
   2.41% at 7,546, steady ~2.8% at 15,918 (NX 0.93% = plausible
   true-death for ranks 100–500k; servfail+timeout 1.87%). unbound-tcp
   status: running/answering; 3 sampled timeout domains (3/5 are `.cn`
   geo-DNS class) resolve through the SAME 5354 forwarder right now in
   1.1–1.8 s — real upstream resolution, not cache. Verdict: slow-but-
   alive domains near the 10 s boundary — the cleanup round's job, NOT
   resolver distress. Operator note appended to the wave monitor.log.
7. **Hygiene**: `/tmp` purged of the stale nxfix sample (§f45; annotated
   resolved in the 08-57 report); disk headroom verified for B's snapshot
   - wave C (61 GB free; snapshots dir is compact at 163 MB for three
     waves — tars compress the db ~6.5×, so the "378 MB per snapshot"
     fear in §f46 is overstated).
8. **Verification**: full `-race ./...` suite green (9/9 packages, twice:
   plain flags and buildflow's exact invocation
   `-parallel=32 -shuffle=on CGO_ENABLED=1`); lint 0 issues on the two
   touched packages; `go build ./...` green.

## b) PARTIALLY DONE

1. **Wave B is ~4% in** (15,918/400,000 at 09:50, ~4.6 dom/s sustained —
   confirms the census.md rate note): gate, cleanup loop, snapshot land
   on wrapper completion (~tomorrow morning). The ~2.8% pass-1 unknown
   hover is above the 2% poller bar but explained (see a6); the D3 gate
   runs POST-cleanup by design.
2. **Buildflow `test-race` step**: failed twice under buildflow while the
   identical `go test` invocation passes directly — root-caused to the
   result cache replaying a failure from a run that was concurrent with
   my own suite (contention artifact; the scary-looking "Doctor checks
   failed" line is EXPECTED stderr from the failing-doctor tests, which
   cmdguard prints to process stdout). Uncached re-run launched; per
   AGENTS.md the durable law stays: verify via direct gate runs while
   the buildflow binary predates the go 1.27.2 floor.

## c) NOT STARTED (blocked on the single-flight or the owner)

1. **T11-poisonfix launch** (§f3): input ready, ~10 min at w4; needs B
   complete + owner go. Immediately after it: `replay --audit-excluded
   <correction-run-ids>` must print `0 poisoned` before wave-A data is
   declared trustworthy (§f4).
2. **Wave C chaining, T15 census report, T17/T19/T20** — unchanged from
   the 08-57 report §c.
3. **Wave watchdog** (§f16): harvested into TODO_LIST; implementation is
   BLOCKED until census-wave.sh can be edited again (post-B).

## d) TOTALLY FUCKED UP (small, but honest)

1. **Ran my own `-race ./...` concurrently with buildflow's dev gate** —
   the classic contention mistake. It produced a phantom cli-package
   failure that the result cache then faithfully replayed in the isolated
   re-run, costing a diagnosis round-trip. The uncached rerun + direct
   green runs settle it; lesson: one verification lane at a time on this
   2-core-ish host, and reach for `BUILDFLOW_NO_RESULT_CACHE=1` when a
   step "fails" suspiciously fast.
2. **First poison-list pass replayed the wrong run for c5** (empty LIKE
   match → `--run ""` → latest = nxfix-c1). Caught by row-count
   mismatch (210 ≠ 15) before any list was written; the snapshot journal
   made the retry free.
3. **The audit unit test's first fixture asserted wrong semantics** —
   I expected a healthy domain last seen in the excluded run to be clean;
   poison is provenance, not verdict, so it must be poisoned. The test
   caught ME, which is the test working; the shipped semantics are the
   correct ones.

## e) WHAT WE SHOULD IMPROVE

1. Census-gate: add the audit invocation as a post-correction stage
   (assert `0 poisoned`) once the poisonfix pass has run once.
2. Consider `--audit-excluded` accepting wave NAMES (resolved via runs)
   to spare operators ULID archaeology.
3. The poller's unknown-rate bar (2%) could carry a per-band expectation
   (ranks 100–500k runs hotter than 1–100k) so warnings need less
   archaeology — note appended to the wave monitor.log documents today's
   numbers as the calibration point.

## f) UP NEXT (ordered)

1. Post-B: verify wrapper's gate/cleanup/snapshot landed; ledger row.
2. Launch tranco1m-a-poisonfix (owner go per the standing decision
   framework); audit → `0 poisoned`; then the supersession-audit claim
   in census.md is TRUE unqualified.
3. Launch wave C; monitor with the same poller discipline.
4. T15 census report (pin excluded runs + supersession accounting).
5. Wave watchdog once census-wave.sh is editable again.
6. §g of the 08-57 report: all three owner questions remain open
   (C-chaining policy, poisonfix timing [recommendation: immediately
   post-B], CI babysitting).

## g) VERIFICATION TRAIL

- `go build ./...` green; `go test -race ./...` 9/9 green (twice, incl.
  buildflow's exact flags); `golangci-lint run` 0 issues on
  internal/scanstore + internal/cli; gofumpt/golines clean.
- CLI smoke of the shipped feature lives in the e2e tests (real scan →
  journal → audit), not against the census db.
- The poisonfix list derivation is reproducible: replay the three runs
  from the snapshot tar with `--format jsonl`, grep `error_kind=nxdomain`,
  sort -u (documented in the census.md ledger row).
