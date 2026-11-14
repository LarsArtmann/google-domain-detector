# Docs-health audit pass: living docs rebuilt, 150+ verdicts, 3 plans archived, CI-red lane finished

**Session:** 2026-11-14 01:38 → 02:46 CET · **Mandate:** "View ALL
2026-* files, execute the docs-health skill, make the six living docs
superb, archive fully-done files." **HEAD at report time: `93b9a40` +
a dirty tree owned by the live sibling session** (test-file splits in
flight: cli_scan_test.go, scanstore_test.go, stream_test.go, new
cli_scan_stream_test.go — their lane, untouched). Two concurrent
sessions wrote INTO this window (the 02-05 closeout report and the
02-18 owner package + RulesetV2 note); their files were integrated,
not duplicated. Wave B RUNNING throughout (single scan confirmed at
02:46; watchdog quiet all session). **5+ commits unpushed** — the
audit's lint fix included; CI has not yet seen it.

## a) FULLY DONE (this session, all gate-verified)

1. **Corpus read** — all 14 open timestamped files at session start
   (7 status, 5 planning, 2 review) plus the living docs; the literal
   `2026-0*` pattern matched zero files, so the read covered every
   `2026-*` file (28 archived ones only where a gate flagged them, per
   the closed-history rule).
2. **CI-red lane finished (code)** — the sibling's wave-names landing
   carried 2 abandoned lint findings (`runReplayAudit` cyclop 15 > 12,
   ineffassign `isWave` at audit.go:160) that had kept CI red on every
   push since ~00:30. Fixed: the duplicated event-store error mapping
   extracted to one `mapAuditError` helper used by both failure paths;
   the bare wave-name fallback stops re-tagging a flag it never reads.
   `golangci-lint run internal/cli/...` = 0, audit-scope tests green,
   full `go test ./...` green, full-repo lint 0 issues.
3. **FEATURES.md rebuilt** — re-inventoried against code: added the
   existence oracle, `enrich`, `replay --since`/`--audit-excluded`,
   `runs --format`, staleness fix, census machinery, zone inputs,
   distribution; corrected PARTIALLY (wave A complete + nxfix, wave B
   running, .ch input ready, .uk owner-blocked); PLANNED shrank to
   asciinema + wave C closeout. Status vocabulary normalized to
   FULLY_FUNCTIONAL / PARTIALLY_FUNCTIONAL / PLANNED.
4. **TODO_LIST rebuilt** — 538 → 123 lines (82% struck done-rows
   deleted; completed work lives in CHANGELOG/reports). Open work
   minted with evidence: RulesetV2 provenance decision, U2/U1 live
   smoke on a census.db snapshot, file-size headroom, crt.sh timer
   verification, post-B verification bundle, scanstore audit tails
   (17-43 §f5/§f12/§f19), go-cqrs-lite patch sweep, STACK go-codec
   row, LSP-noise row, Kith CRM delta confirm (repo: crm).
5. **ROADMAP cleaned** — dropped the shipped Homebrew tap and the done
   IPv6 check; absorbed the parked post-census ideas (enrichment join,
   registrant dedup, prospects mode, journal-metadata evolution).
6. **CHANGELOG completed** — Unreleased gained the three landed-but-
   unlogged user-facing features: `runs --format`, the read-model
   staleness fix (`WaitProjectionsSettled`), and the M365 fingerprints
   (append-only preserved — an initial bad edit that REPLACED existing
   entries was caught and restored within the same pass, see §d1).
7. **www-strip split brain closed** — TODO_LIST + two reports carried
   it as an open owner question while docs/census.md:233 records the
   DECISION (2026-11-12: normalize and scan). TODO row deleted; the
   truth note added to the status index; the reports annotated.
8. **150+ inline report verdicts** — the docs-gates corpus (113
   audit-all-flagged items + 6 freshness stragglers + the 17-43
   §f table's 30 rows): 106 done (hashes/evidence), 7 superseded,
   11 won't-implement/NOT-DO, 13 routed to living rows, 6 pass
   citations — every flagged item dispositioned, none skipped.
9. **Three fully-executed plans archived** — 2026-11-10_{00-50, 01-18,
   09-21} → docs/planning/archived/ with per-row verdicts (527 specs:
   L1 tasks individually evidenced, L2 micro-rows by parent), banners,
   and a bulk manifest. Gates (check-rows --strict + bare-rows) ran
   CLEAN BEFORE the `git mv` — the 2026-10-15 ordering lesson held.
10. **docs/status index rebuilt** — 8 open reports listed, archived/
    pointer with count, cross-report truth notes kept + the www-strip
    note added. No broken links remain.
11. **All five docs-gates PASS** (freshness, audit-all, annotate,
    routing, cadence) — verified repeatedly, including after the
    formatter pass and after the sibling's interleaved edits.
    `nix build` green (vendorHash current). AGENTS.md 219/220 with the
    BuildFlow#56 gotcha updated to tonight's refined evidence.
12. **Three daemon-bundle splits** per the mixed-commit policy — the
    audit lint fix, the hook's AGENTS.md auto-diet, and this pass's
    tail each landed as named commits instead of junk sweeps.

## b) PARTIALLY DONE

1. **CI-green-on-HEAD** — the lint fix is committed and locally
   verified (lint 0, tests green) but **not pushed**: CI has not seen
   it. The release law (tag only after push-triggered CI green) is
   blocked on the push, which is owner-called this session.
2. **Archived-status evidence chains** — verdicts in the 26 annotated
   status reports cite commits/CHANGELOG/reports, but a few earlier
   drafts pointed at old TODO_LIST rows that this pass deleted; the
   evidence survives in CHANGELOG + the cited reports, yet the
   TODO-specific pointers are gone. Spot-checked, not exhaustively.
3. **Concurrent-session reconciliation** — the sibling's in-flight
   edits (ROADMAP/STACK/TODO rows, test splits) were integrated and
   left untouched, but the final TODO_LIST/ROADMAP state at this
   report's writing includes THEIR sweep (1dbf7ce: STACK go-codec row
   landed) — this report does not re-verify their half-finished test
   split.
4. **www-strip residual** — census.md records the 2026-11-12 decision
   (normalize + scan), while the sibling's owner-package ask #4 adds
   "rescan the 26 in wave C". The policy is closed; the rescan
   question was left to the package (not reconciled by this session).

## c) NOT STARTED (correctly sequenced, not dropped)

- Census chain execution: B's gate (ETA ~14:45-16:00) → T14 if FAIL →
  wave C (owner-gated) → poisonfix-25 → closure audit → T15 final →
  closing report — single-flight law held all session.
- RulesetV2 stamp (owner go; the 02-18 decision note is ready).
- DOMAIN_LANGUAGE.md — the doc set's one missing must-have.
- Owner package answers: wave-C launch mode, GPG, BuildFlow#56
  rebuild, Nominet send, poisonfix go.
- Post-census: zone-ch scan, projectionhost settled-API, go-cqrs-lite
  patch sweep, enrichment/registrant-dedup framing, M365 surface.
- Push + CI watch (owner).

## d) TOTALLY FUCKED UP

1. **A CHANGELOG edit replaced existing Unreleased entries instead of
   appending** — drafted against a stale mental model of the file.
   Caught on the next read, restored within one edit. Append-only is
   the format's core law; the failure mode was mine, the recovery was
   fast only because the gate ritual re-read the section.
2. **Struck a line containing a literal `~`** ("ETA ~14:45") — the
   tilde collided with the `~~` markers and mangled the row (the
   annotate tool does not refuse tildes in struck spans). The gate
   caught it; hand-repaired with a tilde-free re-strike. Lesson: scan
   item text for marker-colliding characters BEFORE writing specs.
3. **Fought the formatter three times on one space** — the census.md
   rule-F fix (dprint strips the space before an inline-code span at a
   wrap boundary) failed, failed again, and only stuck when I removed
   the inline code entirely. The third attempt was the first that
   changed strategy; it should have been the second.
4. **A commit message misdescribed its content** — "Reformat AGENTS.md
   paragraphs (dprint rewrap)" for what the hook's agent-md-size
   repair had actually condensed; required a message-only amend.
   Messages must be written from the diff, not from the assumption.
5. **Formatter interleaving churn** — I ran `dprint fmt` between
   annotation batches, which invalidated read-before-edit state twice
   (two edit refusals) and re-wrapped strikes mid-corpus. The canon
   says annotate → format ONCE → re-gate; I paid for the detour.
6. **Started annotating as if 01-45 were the newest report** while the
   sibling session was mid-closeout — docs-gates freshness caught the
   02-05 report mechanically, but a `ls -t docs/status/` before the
   manual reads would have surfaced the live writer immediately.
7. **Guessed table shape once** — generated a 9-cell replacement row
   for a 5-cell table (guessed Imp/Eff/Gate values); caught by my own
   cell-count print before any write landed.

## e) WHAT WE SHOULD IMPROVE

1. **Run docs-gates FIRST in any docs-health pass** — the battery's
   UNHARVESTED list IS the authoritative itemized worklist; reading
   reports manually before it spent context reproducing what the gate
   prints in one call.
2. **Batch external mutators** — one formatter pass after the whole
   sweep, never interleaved; re-read any file the daemon or formatter
   may have touched before editing (the edit tool's staleness check is
   the guard, but planning around it is cheaper than tripping it).
3. **The annotate tool should refuse tilde-bearing struck spans** —
   tonight's mangle class is mechanically detectable at spec time
   (candidate upstream check for docs-gates/annotate.go).
4. **Concurrent-session detection at start** — `ls -t docs/status/ |
   head -3` + a git-log timestamp check before treating any report as
   "newest"; tonight two sessions' outputs interleaved and only the
   gates kept the corpus coherent.
5. **Commit smaller, sooner** — three daemon races hit this session
   because large dirty trees sat while gates ran; per-unit commits
   would have made the bundles self-describing.
6. **Evidence pointers should target artifacts that outlive the
   TODO_LIST** (CHANGELOG entries, reports, commits) — tonight's
   rebuild deleted the rows some verdicts had cited.

## f) UP TO 50 THINGS TO GET DONE NEXT

**Census critical path (single-flight, in order):**

1. Watch wave B to completion (~14:45-16:00); watchdog/heartbeat.
2. On B exit: read the wrapper stderr summary → run-id file.
3. Verify B's auto-gate PASS (`--manifest --expect-skip 96215`).
4. If gate FAIL: T14 cleanup loop + explicit-resume reconcile.
5. Owner go (package ask #1): attended vs auto-chained wave C launch.
6. Owner go (ask #2): stamp RulesetV2 (`v2-2026-11-14`, ~10 min:
   ruleset const + pin test) BEFORE wave C's binary is built.
7. Launch wave C (gdd binary stamped V2, w4/10s, 5354, TCP preflight).
8. Attach a fresh watchdog to wave C.
9. Owner go (ask #3): poisonfix-25 (fresh binary, PROVENANCE stamp,
   audit-derived 25-domain list).
10. Closure proof: `replay --audit-excluded <4 run IDs>` → 0 poisoned.
11. Verify wave C's + poisonfix's snapshot tars + sha256 exist; restore-
    verify one (integrity_check + counts); census.db disk headroom.
12. T15 final whole-census distribution (journal-derived, replay cmd
    pinned) in reports/census/REPORT.md.
13. Closing status report + index row + (on push) CI green.
14. Verify the 04:37 crt.sh timer run landed in-repo (plausibility
    guard ≥100k .de rows) — today's first legit post-fix run.
15. `pgrep -f 'gdd[^ ]* scan'` = exactly ONE scan through C + poisonfix.

**Code (single-flight-free):**
16. U2/U1 live smoke on a census.db SNAPSHOT: audit wave-names vs
per-run ledger counts; `runs --format jsonl` full IDs; pin the
ULID-vs-lowercase-wave-name edge.
17. Land the sibling's in-flight test split (their lane — coordinate,
don't collide): cli_scan_stream_test.go et al.
18. File-size headroom: scanstore_test.go (346/350) and
cli_scan_test.go (344/350) — split before the next addition.
19. Full `-race ./...` once on the settled pushed HEAD (one lane).
20. Push the queued commits (5+) and watch CI green on the exact HEAD
(release law) — owner-called.
21. STACK.md go-codec row — verify the sibling's swept row (1dbf7ce)
matches the stack-selection decision record format.
22. scanstore audit tails: read `sqliteengine/v4` `MapUpdate` source;
verify memory/sqlite fold dispatch share the task snapshot; pin
the Recorder sequential-dispatch contract with a test.
23. Clear the gopls `infertypeargs` infos or quiet the integration
(cli.go ×2, scanstore.go ×3) + the stale golangci LSP warnings.
24. io-surface info: internal/ranges/ranges.go:265 os.ReadFile TOCTOU
(open + handle instead of stat-then-read) — info severity, decide
fix-or-accept.
25. shellcheck warnings in census scripts (census-gate.sh SC2034
unused var; census-wave.sh quote findings) — tidy or suppress with
rationale.

**Docs:**
26. BUILD docs/DOMAIN_LANGUAGE.md (the missing must-have: verdicts,
evidence-only payloads, supersession, waves, rulesets — from
classify/scanstore usage).
27. Audit upkeep: re-shoot docs/review/libs index render proof.
28. Audit upkeep: re-audit go-cqrs-lite/cqrs-htmx verdicts vs current
pins (the sibling's sweep touched go-cqrs-lite.md — re-check).
29. Verify the sibling's ROADMAP sweep (1dbf7ce) kept the ownership
matrix clean (no TODO/ROADMAP duplication re-introduced).
30. CHANGELOG Unreleased coherence before the next tag (the daemon
keeps landing entries; verify vs git log at tag-prep time).
31. Add the audit-round-trip recipe to docs/census.md (declined for
AGENTS.md at cap — census.md owns recipes) if a second session
needs it.
32. Re-run the full docs-gates battery after the sibling's test-split
landing (their edits touch TODO_LIST prose the gates replay).

**Owner decisions (the 02-18 package, reply by number):**
33. #1 wave-C launch mode (attended recommended).
34. #2 RulesetV2 stamp (YES recommended).
35. #3 poisonfix-25 launch (YES recommended).
36. #4 www-strip rescan-in-C — reconcile against the 2026-11-12
census.md decision first (policy decided; rescan is the open bit).
37. #5 GPG signing policy. **→ routed - owner-package ask #5 (docs/planning/2026-11-14_02-18-owner-decision-package.md); the TODO_LIST owner-gates pointer covers it**
38. #6 BuildFlow#56 fleet rebuild approval (the hook still fails valid
commits on the tidy demand).
39. #7 Nominet .uk email send (draft ready).
40. #8 M365 surface + DKIM probe (post-census, render-side tally).

**Post-census / parked:**
41. Kith CRM: confirm delta retirement onto `replay --since` (repo: crm).
42. go-cqrs-lite projectionhost settled-API (the repo-local
WaitProjectionsSettled is the reference).
43. go-cqrs-lite patch sweep + foldByEvent re-verification.
44. zone-ch scan (2.66M apexes) after Tranco frees single-flight.
45. asciinema demo / GIF for the README.
46. io-history-ratchet `--profile-io` investigation (56 vs median 10).
47. TCP contention measurement post-B (`incoming/outgoing-num-tcp`).
48. Poller rank-band unknown-rate expectations (calibrate on B pass-1;
bound the "none" share).
49. Lead scoring / daily delta timer framing (ROADMAP direction 3).
50. Next docs-health pass after the census closes (the archive ritual
for the 8 open reports; the 02-05/02-18 generation will age).

## g) QUESTIONS I CANNOT FIGURE OUT MYSELF

1. **Push the 5+ queued commits now?** They contain the lint fix that
   un-CIs the branch, the docs-health pass, and the sibling's swept
   rows. I verified locally (build/vet/lint/tests/nix green) but the
   release law wants CI green on the exact pushed commit — I don't
   push without your word, and the sibling session may still hold
   in-flight work (5 dirty files at 02:46).
2. **www-strip ask #4 vs the recorded decision:** census.md:233 decided
   "normalize and scan" on 2026-11-12 (the 26 domains scanned fine in
   wave A), yet the 02-18 owner package asks whether to RESCAN them in
   wave C. Do you want the rescan, or should ask #4 be closed as
   answered by the standing decision (leaving only the policy as
   decided)?
3. **Should I BUILD docs/DOMAIN_LANGUAGE.md in the next window** (the
   doc set's one missing must-have)? If yes: full ubiquitous-language
   glossary (verdicts, evidence-only payloads, supersession, waves,
   rulesets, oracle semantics) or a minimal seed to grow per session?
