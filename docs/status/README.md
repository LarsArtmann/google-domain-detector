# Status Reports Index

Point-in-time session reports, newest first. Each is a snapshot of its
moment — verify claims against current code before citing (reports carry
their HEAD when known; see the "verified against" convention in
TODO_LIST).

| Report                                                                              | Scope                                                                                                                                                                                                                   |
| ----------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| [2026-11-13_02-05](2026-11-13_02-05_false-nxdomain-root-cause-and-correction-wave.md) | **False-NXDOMAIN ROOT CAUSE: Go conflates NXDOMAIN and NODATA into one IsNotFound** — scanner stamped every live no-MX/no-evidence domain dead (~90% of wave A's 22,010 NX false, 27/30 alive at Quad9); fixed via rcode-level existence oracle (commit 7b45a1b, binary gdd-20261113, lint-zero, tests green); **tranco1m-a-nxfix correction wave** (22,136 domains) running; 01-04 report's burst theory FALSIFIED and annotated |
| [2026-11-13_01-04](2026-11-13_01-04_census-plan-execution-and-resolver-forensics.md) | Census plan execution: scanstore review (canonical), Ruleset/--wave provenance stamps (live in journal), census-slice pipefail fix, orphan-resolver kill, **false-NXDOMAIN pilot forensics** (12/12 sampled NXDOMAINs false under burst — root cause superseded by the 02-05 report); §g owner questions |
| [2026-11-12_18-45](2026-11-12_18-45_wave-a-complete-resume-hole-root-caused.md)     | **Wave A complete** (health PASS, unknown 1.53%); **2,731-domain resume hole** root-caused (`--resume latest` ← aborted soak3) and gated shut (gate `--manifest` exit 4; wrapper fresh/explicit-retry); 90,834 proven fictional; wave B held |
| [2026-11-12_04-15](2026-11-12_04-15_self-review-census-hardening.md)                | Self-review of the census hardening session; input-count correction (100,000 not 90,834); snapshot routine + WARNING poller landed; 3 owner questions (§g)                                                              |
| [2026-11-11_22-05](2026-11-11_22-05_census-hardening-while-wave-a-runs.md)          | Census hardening while wave A runs: poller WARNING, provenance stamp, audit-freshness CI, cmdguard v5.1.0 re-pin, snapshots                                                                                             |
| [2026-11-11_17-43](2026-11-11_17-43_cqrs-event-sourcing-audit-status.md)            | cqrs/event-sourcing audit status (sibling lane)                                                                                                                                                                         |
| [2026-11-11_15-27](2026-11-11_15-27_full-status-wave-a-running.md)                  | Full status while wave A runs (§g + §f); NOTE: carried the fictional 90,834 denominator — corrected 2026-11-12                                                                                            |
| [2026-11-11_14-44](2026-11-11_14-44_census-wave-a-self-review.md)                   | Wave-A launch self-review; NOTE: first appearance of the underved 90,834 figure — root-caused 2026-11-12 (no primary source exists)                                                                       |
| [2026-11-11_12-35](2026-11-11_12-35_wave-a-launch.md)                               | E10 closed, soak ladder, wave A launched detached                                                                                                                                                                       |
| [2026-11-11_02-00](2026-11-11_02-00_e24-lint-zero-e33-coverage-fleet-conflict.md)   | E24 lint sweep / E33 coverage (sibling lane)                                                                                                                                                                            |
| [2026-11-10_13-27](2026-11-10_13-27_v010-released-race-flake-killed-lane-landed.md) | **v0.1.0 RELEASED** (tag + push ×2 + GitHub Release + proxy-verified); race flake root-caused (startup-cancel → deterministic triggers); E19/E26/E27/T15/T16/E30 landed; negative-cache finding; CI-red-on-tag incident |
| [2026-11-10_10-11](2026-11-10_10-11_v3-spine-race-blocked-session-status.md)        | Plan v3 spine: E3/E4/E5 landed, E1 replication (best run), release blocked on a flaky cancel test (since fixed deterministically — see 2026-11-10 E1 finding below)                                                     |
| [2026-11-10_07-46](2026-11-10_07-46_t5-validated-read-models-cli-polish.md)         | T5 validation, show/runs read models, CLI polish; **nxdomain-bucket conclusion retracted** — see the correction note at the top                                                                                         |
| [2026-11-10_06-58](2026-11-10_06-58_scale-tier-execution-session-status.md)         | Scale-tier execution session                                                                                                                                                                                            |
| [2026-11-10_06-09](2026-11-10_06-09_toolship-and-first-mass-scan.md)                | Toolship + first 63k-domain mass scan (Tranco .de/.ch/.uk)                                                                                                                                                              |
| [2026-11-10_01-49](2026-11-10_01-49_full-plan-execution-session-status.md)          | Plan v2 full execution session                                                                                                                                                                                          |
| [2026-11-10_01-12](2026-11-10_01-12_event-sourced-scans-session-status.md)          | Event-sourced scans (scanstore) session                                                                                                                                                                                 |
| [2026-11-10_01-12 planning](2026-11-10_01-12_planning-run-pareto-plan-review.md)    | Planning run / pareto plan review                                                                                                                                                                                       |
| [2026-11-10_00-36](2026-11-10_00-36_library-audit-session-review.md)                | Library audit session (22 LarsArtmann libs)                                                                                                                                                                             |

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
