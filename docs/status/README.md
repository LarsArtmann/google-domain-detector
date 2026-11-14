# Status Reports Index

Point-in-time session reports, newest first. Each is a snapshot of its
moment — verify claims against current code before citing. Reports with
every item resolved move to [archived/](archived/) (its README carries
the bulk-archive manifests); the files below are still-open snapshots.

| Report                                                                                | Scope                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                        |
| ------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| [2026-11-14_01-45](2026-11-14_01-45_full-status-resumed-session-a-to-g.md)            | **Full status (a–g) of the resumed session** — reconciliation, runs --format, the read-model staleness discovery+fix, M365 fingerprints, FormatsForShape/TXT-truncation closures, owned process misses (02-15 filename lie → renamed, repo-wide format in a shared tree, late experiment cleanup), 40 next items + 3 owner questions |
| [2026-11-14_01-38](2026-11-14_01-38_runs-format-staleness-fix-m365-fingerprints.md)   | **Code window: runs --format, staleness fix, M365** — `runs --format` full enum with FULL run IDs on every data format (footer-leak-into-csv fixed in runs AND the supersession audit); **read-model staleness root-caused** (folds live in projection workers, not dispatch — `Open`+immediate query deterministically saw an empty ledger; `WaitProjectionsSettled` wired into runs/show/enrich, `--resume` proven immune, library TODO row queued); M365 fingerprints as evidence-only (verdict invariance pinned); FormatsForShape closed as superseded by the T22 pin; TXT-truncation row struck (closed-with-evidence since 00-03); sibling lane handoffs (audit wave-names + file split in flight, branch red only on their 2 WIP findings); wave B at 38%, 0 alarms |
| [2026-11-13_21-15](2026-11-13_21-15_wave-b-watch-git-reconciliation-handoff-audit.md) | **Session resume: git reconciliation + wave-B watch** — reviewed and pushed the concurrent cqrs-lint session's 4 daemon-swept commits (go-codec indirect→direct promotion verified tidy-stable; F005/F007 anchor move matches the first-file rule; AGENTS diet verified not-lost: detail lives at firing sites + CHANGELOG + TODO row); build/vet/tidy/scanstore gates green; full TODO_LIST reconciliation = 19 open rows, handoff drift documented; wave B attempt 2 at 18% (~16:00 Nov 14 ETA), watchdog quiet; 40 next items + 3 owner questions |
| [2026-11-13_19-50](2026-11-13_19-50_cqrs-lint-triage-zero-findings.md)                | **cqrs-lint driven to zero** (the §f1 debt from 10-23): 14 findings → 2 real fixes (F031 ListRuns silently capped at 100 rows → `WithLimit(0)`; C023 start-failure close errors now reported) + 12 rationale-at-site suppressions; sibling's cborsanitize.go became the F005/F007 package anchor mid-triage (handled); stale-suppression + anchor mechanics researched at source; file-size debt (9 files >350 lines) queued as a row; concurrent-session collision survived cleanly |
| [2026-11-13_17-20](2026-11-13_17-20_hostile-txt-poison-wave-b-relaunch-docs-reconciliation.md) | **Hostile-TXT journal poison root-caused and fixed end-to-end** (invalid-UTF-8 TXT → CBOR decode poison + JSON v2 render death → scan-boundary sanitizer + CBOR decode repair, fixture-pinned; wave B wedged 4.5 h, watchdog caught it, SIGINT drained, attempt 2 resumed on gdd-20261113b); **supersession audit ran: poisonfix 948 → 25**; docs-health pass (60 annotations, 28 reports archived, resolv.conf Err() bug fixed, order-flaky audit test fixed); AGENTS diet 349→222; .ch input (2.66M apexes); T15 report; 17 open rows + §g |
| [2026-11-11_17-43](2026-11-11_17-43_cqrs-event-sourcing-audit-status.md)              | **CQRS/event-sourcing audit status** (sibling lane): scanstore verified conformant to the go-cqrs-lite canon (15-point evidence table, live journal round trip); §f = 30 harvest-ready rows (many since resolved: cqrs-lint triage, M365, T16); §g owner questions on causation/actor metadata + EventCatalog timing |
| [2026-11-10_19-27](2026-11-10_19-27_gcloud-expertise-sales-loop-session.md)           | **GCloud-expertise sales-loop session** (owner's sales lane): strategy answer + brainstorm; most §f items routed "won't implement — owner's sales lane (crm/docs/sales/)"; remaining repo-adjacent ideas parked in ROADMAP (prospects mode). Excluded from archiving by the 2026-11-13 bulk pass (owner lane) |

Archived history: 28 fully-resolved reports (2026-11-10 through
2026-11-13) live in [archived/](archived/README.md) with their manifest.

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
- **www-strip policy is DECIDED (2026-11-12):** normalize and scan —
  docs/census.md §"www-strip policy". Reports listing it as an open
  owner question (21-15 §g3, 01-38 §g2, 01-45 §f28) predate the decision
  and are annotated accordingly.
