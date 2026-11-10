# Status Reports Index

Point-in-time session reports, newest first. Each is a snapshot of its
moment — verify claims against current code before citing (reports carry
their HEAD when known; see the "verified against" convention in
TODO_LIST).

| Report | Scope |
| ------ | ----- |
| [2026-11-10_10-11](2026-11-10_10-11_v3-spine-race-blocked-session-status.md) | Plan v3 spine: E3/E4/E5 landed, E1 replication (best run), release blocked on a flaky cancel test (since fixed deterministically — see 2026-11-10 E1 finding below) |
| [2026-11-10_07-46](2026-11-10_07-46_t5-validated-read-models-cli-polish.md) | T5 validation, show/runs read models, CLI polish; **nxdomain-bucket conclusion retracted** — see the correction note at the top |
| [2026-11-10_06-58](2026-11-10_06-58_scale-tier-execution-session-status.md) | Scale-tier execution session |
| [2026-11-10_06-09](2026-11-10_06-09_toolship-and-first-mass-scan.md) | Toolship + first 63k-domain mass scan (Tranco .de/.ch/.uk) |
| [2026-11-10_01-49](2026-11-10_01-49_full-plan-execution-session-status.md) | Plan v2 full execution session |
| [2026-11-10_01-12](2026-11-10_01-12_event-sourced-scans-session-status.md) | Event-sourced scans (scanstore) session |
| [2026-11-10_01-12 planning](2026-11-10_01-12_planning-run-pareto-plan-review.md) | Planning run / pareto plan review |
| [2026-11-10_00-36](2026-11-10_00-36_library-audit-session-review.md) | Library audit session (22 LarsArtmann libs) |

## Cross-report truth notes

- **2026-11-10 E1 finding (supersedes 07-46 tail conclusions):** stale
  negative-cache entries make NXDOMAIN-class results through a shared
  cache untrustworthy long after the resolver recovers (11/12 sampled
  "nxdomain" alive in the 10:10 run). Positive verdicts are unaffected.
  Details: AGENTS.md gotcha + reports/2026-11-10-e1-validation/ (local).
- **The flaky `TestScanCancelMidRunDrainsEventStore` failure named in the
  10-11 report was a flaky TEST, not a product race:** a wall-clock
  100ms cancel could land during event-store startup (a clean failure by
  design). Fixed with a deterministic trigger; full -race gates green
  twice consecutively (2026-11-10).
