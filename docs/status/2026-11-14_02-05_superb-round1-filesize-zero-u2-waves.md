---
date: 2026-11-14 02:05
session: SUPERB plan execution round 1 — file-size debt to zero, U2 shipped
status: DONE
tags: [census, buildflow, file-size, audit-excluded, cli-split]
---

# 02-05 — SUPERB Round 1: File-Size Debt Zeroed, `--audit-excluded` Waves, Census Steady

**Session window:** 2026-11-14 01:01–02:05 CET · **Mandate:** execute the
SUPERB closeout plan (`docs/planning/2026-11-14_00-42_…`) end to end.

## Outcome in one line

Every file-size and go-structure finding is FIXED and gate-verified (both
steps pass clean at 02:05); U2 (`--audit-excluded` wave names) shipped with
tests; wave B continues healthy at 3.8/s; U1 and the classify half of U5
landed by sibling sessions and were integrated, not duplicated.

## a) FULLY DONE (this session, all gate/test-verified)

1. **Session-start checks** — CI green on a62b667; exactly one scan
   (B attempt-2, pid 2327170); sibling work in flight identified
   (`enrich.go` wait-wiring, `runs.go` U1) and left untouched.
2. **U2 `--audit-excluded` wave names** — bare wave names and `wave:NAME`
   expand to their run IDs; unknown tokens are usage errors (exit 2);
   expansion is chronological and dedup'd; mixed ID+name works.
   - New scanstore API `RunSummaries(ctx, sys)` — journal-only run
     summaries (first-started order); the audit stays Boot-composed and
     read-model-independent per the replay law.
   - cli/replay.go audit section extracted to cli/audit.go (218 lines);
     delta family to cli/delta.go (93).
   - Tests: `TestReplayAuditExcludedAcceptsWaveNames` (4 invocation
     shapes × attribution + dedup), `TestReplayAuditExcludedRejectsUnknownTokens`
     (exit 2 both forms), `auditWaveTestStore` fixture.
   - Docs: README audit section + cheat-sheet line, CHANGELOG Unreleased,
     docs/census.md usage, TODO_LIST row struck.
3. **Q1a scanstore split** — scanstore.go 378→262 (`sink.go` 126: Sink,
   runRecorder, retry constants); scanstore_test.go 586→346 (`sink_test.go`
   90, `soak_test.go` 102: G4 benchmark, `stream_test.go` 76: T8.9 gate).
4. **Q1b replay split (redefined)** — the plan targeted scanstore/replay.go
   (347, already under bar); the real offender was cli/replay.go 554 →
   255 after moving the audit block (audit.go) and delta family (delta.go).
5. **Q1c cli.go split** — 769→237 root wiring; `scan.go` 330 (runScan body,
   budget, chunks, streaming), `output.go` (render helpers), `eventstore.go`
   (attachEventStore + sinkDepth), `resume.go` 55 (resume reconciliation +
   input accounting), `ranges.go` 36 (range loading).
6. **Q1d test-file splits** — cli_test.go 1762→217 + 7 family files
   (scan 344, scan-budget 208, scan-eventstore ~146, scan-resume 234,
   enrich 154, replay 288, show 164); coverage_test.go 576→51 + 4
   (ranges 182, replay 108, enrich 79, scan 212); scan_test.go 476→142 +
   domain 151 + run 213; ranges_test.go 356→234 + cache 98; render_test.go
   357→251 + json 118.
7. **classify.go split** (after sibling committed `5fab996`, ownership
   resolved) — 364→314 + `matchers.go` 48 (SPF include probing, M365 MX
   check, normalize/isSubdomainOf helpers).
8. **AGENTS.md ≤220** — 230→219 via lossless compressions (NXDOMAIN, bash
   edit law, DoT, metaengine fold, UDP-outage, crt.sh, snapshot, instant/
   schema, docs-gates, read-models, go-directive, session-start, gh-DNS,
   mixed-commit). Every distinct fact preserved.
9. **C1 census spot-check** — B steady 3.8/s (115,791 at 01:23), heartbeat
   every 30 min, zero WATCHDOG alarms, run-id intact, snapshots flowing.
10. **Verification wall** — `go test ./...` 9/9 packages green (repeatedly),
    gofmt clean, go vet clean, `file-size-check` PASS (0 findings, 02:05),
    `go-structure-linter` PASS (0 findings, 02:05).

## b) PARTIALLY DONE

- **Full buildflow gate end-to-end after the last fixes** — the 01:46 full
  run had exactly 2 error findings (classify.go 364, AGENTS.md 230); both
  are now fixed and their individual steps pass, but a fresh FULL gate run
  (all ~60 steps, with tests) has not been re-executed post-fix.
- **U5 M365/Exchange fingerprints** — the sibling implemented classify
  (mx_microsoft/spf_microsoft + tests) and committed `5fab996`; NOT done:
  ruleset provenance decision (CurrentRuleset is still V1 — the new
  evidence kinds landed without a V2 stamp), user-facing surface (Counts
  tally/verdict), DKIM microsoft selector probe, README/FEATURES copy.
- **U1 `runs --format` jsonl** — sibling completed and committed (full-ID
  formats test present); I verified presence only, did not re-test the
  smoke-on-census.db sub-item.
- **U2 live smoke on census.db** — wave-name resolution is test-verified
  on fixture stores; not yet run against the live census.db (C5 ritual).

## c) NOT STARTED (from the SUPERB plan, in plan order)

- U3 (T15 distribution section — blocked on B exit), U4 (rank-band
  calibration), Q2 (TXT truncation root-cause), Q3 (libs render proof +
  verdict re-audits), Q4 (ops ritual bundle: crt.sh 04:37 verify, snapshot
  restore check, full `-race` on pushed HEAD, CI watch), Q5 (TODO_LIST
  hygiene cycle incl. duplicate file-size rows + E27 close), Q6 (STACK.md
  go-codec row, CHANGELOG coherence, describe battery), R1 (asciinema
  demo), R2 (io-history `--profile-io`), R3 (zone-ch prep), R4 (CRM
  framing), R5 (owner-decision doc — the 6 asks), R6 (hygiene tail),
  C2–C5 (B gate verify → owner-gated C launch, poisonfix, closure proof).

## d) TOTALLY FUCKED UP (and how it was recovered)

1. **Orphaned function body in scanstore_test.go** — my edit replaced a
   test header but left the body dangling; caught by gofmt/build, recovered
   by truncating to the comment and recreating `stream_test.go` cleanly.
2. **Script-inserted unquoted import** (`github.com/...` bare path) in
   coverage_scan_test.go — my import-inserter mangled the v5 alias branch;
   caught by build, fixed by hand-editing the import block.
3. **AGENTS.md tug-of-war** — three compression rounds because the daemon
   kept committing additions between my edits (230→223→224→222→220→219);
   two of my earlier "compressions" saved zero lines due to miscounted
   wraps. Cost: ~4 extra edit/verify cycles.
4. **U2 first design used `WaitProjectionsSettled` on a Boot-ed store** —
   workers never start without `sys.Start`, so the wait burned its full
   60 s budget and failed the test. Root-caused (Boot vs Open law), and
   re-designed to journal-only resolution, which is strictly more correct.
   Cost: one 60 s test cycle + a redesign.
5. **One phantom cli test FAIL** after the gofmt pass — the documented
   host-contention flake (scan + concurrent compiles); two consecutive
   greens on identical code confirmed it. Cost: one diagnosis round.

## e) WHAT WE SHOULD IMPROVE

- **Mechanical splits should run as ONE vet-converge batch** across all
  touched packages from the start (per-file loops multiplied iterations).
- **Line-count edits need post-edit recount in the same command** — never
  trust a wrap-count made while writing the replacement.
- **Boot vs Open is now a recurring trap** — the AGENTS.md read-models
  entry covers it; consider a `scanstore` doc comment on `Boot` naming
  `WaitProjectionsSettled` as Open-only explicitly.
- **Daemon-vs-human line budgets on AGENTS.md** — the daemon adds gotchas
  faster than gates allow; each new entry should land pre-compressed.
- **Ownership handshake** — sibling work is detected by mtime + git log;
  a one-line claim in monitor.log or a WORKING_ON.md would remove the
  guesswork (this session guessed right twice, but it was luck-adjacent).

## f) NEXT (ranked, ≤50)

1. Re-run the FULL buildflow gate end-to-end on the current tree.
2. `buildflow format` once (formatter-fingerprint-drift warn).
3. `buildflow -s nix-hash-fix --fix` (stale vendorHash warn) + `nix build`.
4. C1 continued: heartbeat watch until B exits (ETA ~14:45–16:00).
5. C2 on B exit: wrapper stderr summary → run-id; gate `--manifest --expect-skip`.
6. U4: extract B pass-1 unknown-rate by rank band; set poller expectations.
7. U2 live smoke: `replay --audit-excluded wave:tranco1m-a --format jsonl` on census.db vs the ledger's per-run counts.
8. U5 decision note: RulesetV2 bump (or V1-extension rationale) — owner ask.
9. U5 surface decision: Microsoft tally in run summary / render (owner ask).
10. U3 (post-B): T15 distribution section, journal-derived, replay cmd pinned.
11. Q4: verify the 04:37 crt.sh timer landed in-repo with rows ≥ floor.
12. Q4: full `-race ./...` once on pushed HEAD (single lane).
13. Q4: `gh run list` CI green on the daemon-swept HEAD commits.
14. Q4: snapshot restore-verify one post-B tar (integrity_check + counts).
15. Q5: TODO_LIST hygiene (docs-gates audit-all sweep, merge the duplicate file-size rows, close E27, strike done rows).
16. Q6: STACK.md go-codec row (cborsanitize made it direct).
17. Q6: CHANGELOG Unreleased coherence vs daemon-landed entries.
18. Q6: `describe` battery eyeball post-resume/wave flags.
19. Q3: docs/review/libs index.html render proof re-shoot.
20. Q3: go-cqrs-lite verdict re-audit vs current pin.
21. Q3: cqrs-htmx verdict re-audit.
22. Q2: reproduce the cloudflare TXT "cannot unmarshal" class from wave logs; root-cause the decoder path; fix or file.
23. R5: owner-decision doc — www-strip, B→C auto-chain, GPG, BuildFlow#56 rebuild, Nominet email, poisonfix go (one doc, one ask batch).
24. R3: zone-ch manifest + gate params + post-C slot note.
25. R1: asciinema demo script (60-domain TTY scan) + GIF + README embed.
26. R2: io-history `--profile-io` (56-file offender).
27. R4: CRM delta-retirement confirm + enrichment/registrant-dedup ROADMAP entries.
28. R6: check the 2 unarchived status reports for closure.
29. R6: quiet stale LSP golangci diagnostics (classify/coverage_scan ghosts above).
30. C3 (owner go): launch wave C via census-wave.sh, GDD_PREFLIGHT_TRANSPORT=tcp.
31. C4 (owner go): poisonfix-25 fresh binary + PROVENANCE stamp + sha256.
32. C5: `--audit-excluded` 4 runs → 0 poisoned; ledger row; snapshot.
33. Post-B: verify B's completion snapshot tar + .sha256 exist and restore.
34. Push verification: daemon-swept commits present on BOTH remotes.
35. CI: watch the push-triggered run on the exact HEAD (release law).
36. T14 contingency: if B's gate FAILs, triage unknowns before C.
37. Confirm T16 replay-delta coverage for the CRM diff logic (R4 detail).
38. Decision: DKIM microsoft selector probe — plan or park (with ruleset question).
39. Decision: `MicrosoftVerdict()` method + Counts.MS tally — now or post-census.
40. Check census.db disk headroom for wave C + zone-ch.
41. Wave-name UX: consider `runs` output showing wave names more loudly (already in provenance suffix — verify).
42. Audit resolver edge: token = wave name that equals a run ID prefix (impossible: ULID vs lowercase — pin with a test comment).
43. scanstore_test.go at 346 — pre-split the next addition (headroom 4 lines).
44. cli_scan_test.go at 344 — same (headroom 6).
45. Move auditTestStore + audit fixtures fully into audit_test.go (cli_test.go slimming, optional).
46. Add a session-start `pgrep` single-flight guard to the ritual checklist (done manually this session).
47. Consider `WaitProjectionsSettled` integration test for runs/show/enrich commands (sibling added projections_test — verify coverage).
48. PROVENANCE.md update when wave C's binary is stamped (C3.1).
49. Closing status report + index after C5 (the census closeout ritual).
50. Park: projectionhost settled-API library fix (TODO row exists, post-census).

## g) QUESTIONS FOR THE OWNER (cannot self-answer)

1. **Ruleset provenance for the M365 fingerprints:** the sibling landed
   `mx_microsoft`/`spf_microsoft` under `CurrentRuleset = RulesetV1`
   (no bump). Ground rule 3 says new detection rules = NEW Ruleset value.
   Do you want a `RulesetV2` stamp before wave C launches (my
   recommendation — wave A/B journals predate Microsoft evidence), or do
   you consider M365 evidence part of V1's intent?
2. **Microsoft evidence surface:** evidence-only (current) vs user-facing
   (Counts tally in the run summary line, render column, a
   `MicrosoftVerdict()`)? This changes the CRM handoff shape.
3. **B→C chain authorization:** when B exits and its gate passes, do you
   pre-authorize wave C + poisonfix-25 launches (C3/C4) now, or decide
   per-ask after the R5 decision doc lands?

## Cross-references

- Plan: `docs/planning/2026-11-14_00-42_SUPERB-census-closeout-pareto-plan.md`
- Sibling landings this window: `5fab996` (M365 fingerprints),
  `0ac08dc` (runs.go WaitProjectionsSettled + projections.go), earlier U1 commits
- Prior report: `docs/status/2026-11-13_19-50_*` (cqrs-lint zeroing)
