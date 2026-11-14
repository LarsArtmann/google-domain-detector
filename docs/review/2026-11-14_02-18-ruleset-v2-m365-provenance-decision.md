# RulesetV2 decision: the M365 fingerprints need a stamp before wave C

**Date:** 2026-11-14 02:18 CET · **Decider:** owner · **Input:** the
`5fab996` M365 fingerprint commit (sibling session, 2026-11-14 ~01:30) and
the classify ruleset contract (`internal/classify/ruleset.go`).

## Question

`5fab996` added two evidence kinds (`mx_microsoft`, `spf_microsoft`) but
left `CurrentRuleset = RulesetV1`. Wave C has not launched. Stamp a
RulesetV2 before it does, or run wave C under V1?

## Facts (all verified in code, not inferred)

1. The ruleset contract (`ruleset.go`, the type's own doc) reads:
   "Any change to classification behavior (**new/changed evidence
   kinds**, weights, verdict logic) MUST bump CurrentRuleset in the same
   commit — a silent mid-census change would poison every downstream
   delta (census plan guards 1+2)." New evidence kinds are explicitly in
   the MUST list. The bump is enforced as an explicit review gate:
   `TestCurrentRulesetPinned` fails on any drift, forcing a conscious
   pin update.
2. Stamping happens at RECORD time: `scanstore.go` writes
   `classify.CurrentRuleset` into every `scan.started` and
   `domain.observed` event (provenance, never verdicts). The binary that
   runs a wave decides the stamp. Old journals keep V1 forever —
   append-only, no migration, no rewrite.
3. The delta guard `warnRulesetMismatch` (replay.go, used by `--since`
   and the audit) fires only when the two runs' stamps DIFFER. Under a
   permanent V1 it can never fire: the protection the contract exists to
   provide is disarmed for exactly the comparison the census will make
   most — B (pre-M365) vs C (post-M365).
4. Wave A and wave B journals predate `5fab996`: they contain ZERO
   Microsoft evidence by construction. That absence is detector
   capability, not market fact.
5. Google-side behavior is provably unchanged by `5fab996`: the platform
   boundary is pinned by tests (Microsoft evidence never upgrades or
   moves a Google verdict; pure-M365 domains detect nothing). Verdict
   weights and logic are untouched.

## Options

### A. Stamp RulesetV2 before wave C (recommendation)

`RulesetV2 = "v2-2026-11-14"`: same Google rules as V1, plus the two
Microsoft evidence kinds. Wave C, poisonfix-25, and every later run
record V2.

- **Cost:** two lines (`ruleset.go` const + the pin test), a test run,
  and one consequence: B→C `--since`/audit comparisons print the
  cross-ruleset warning — which is CORRECT, because they genuinely mix
  detector capabilities.
- **What it buys:** the delta guard works again; every future analyst
  can distinguish "domain adopted M365" from "detector learned to see
  M365"; the contract is honored instead of silently violated.
- **Google-side comparability is preserved either way:** V1→V2 changes
  no Google rule, weight, or verdict path (fact 5), so Workspace/GCP
  adoption numbers stay comparable across A/B/C. Only Microsoft-side
  counts are "new capability" — A/B show structural zeros.

### B. Stay V1, document V1-as-extension

Rationale would be: evidence-only additions cannot move any Google
verdict, so census conclusions are unaffected; a bump makes every
cross-wave comparison print a warning (noise).

- **Fatal flaw:** it voids the contract's own MUST for "new evidence
  kinds" and permanently disarms `warnRulesetMismatch` (fact 3). The
  "poison every downstream delta" failure mode the contract names is
  exactly what a V1-stamped wave C creates for Microsoft-side analysis.
- Warning noise is not a real cost: it fires only on cross-wave deltas,
  is one stderr line, and is true.

## Recommendation

**Option A.** Bump in a commit BEFORE wave C's binary is built, so the
wave ledger records the clean split: A/B = v1-2026-11-10, C+ =
v2-2026-11-14. Also stamp it in the wave-C `PROVENANCE.md` entry and in
`REPORT.md`'s run inventory (T15) so the distribution tables label
which runs saw Microsoft evidence.

## Adjacent question: the M365 user-facing surface (report §g2)

Separate from provenance, and NOT blocking wave C:

- The evidence already flows end-to-end: `DomainObservedPayload.Evidence`
  carries the kinds, the table's Evidence cell and the compact JSON DTO
  (`Evidences`) already render them. No schema change is needed to "see"
  M365 today.
- What is missing is aggregation only: a `Counts` tally (would extend
  `ScanCompletedPayload.Counts` — journal payload schema v2, mid-census
  = no), a `MicrosoftVerdict()` accessor (verdict-shaped derivation over
  evidence-only records — a product decision, not a gap), or a
  render-side summary column (presentation-only, safe anytime).
- Recommendation: **post-census.** Land the render-side tally or the
  Counts v2 extension after wave C + poisonfix close, chosen then; keep
  the DKIM `microsoft` selector probe parked with it (same
  evidence-vs-verdict tradeoff, stronger probe cost).

## Execution plan once approved (≈10 minutes)

1. `ruleset.go`: add `RulesetV2` with the doc comment above; flip
   `CurrentRuleset`.
2. `ruleset_test.go`: update the pin to V2 (the test's failure message
   already scripts this review).
3. `go test ./internal/classify/ ./internal/scanstore/` (provenance
   round-trip pins stamp the new const automatically).
4. Commit message states the contract clause (new evidence kinds) per
   the bump policy; `PROVENANCE.md`/`REPORT.md` notes ride with the
   wave-C launch steps (C3), not this commit.
