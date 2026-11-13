> **Live remainder:** every item resolved as of the 2026-11-13 docs-health pass (docs-gates green); open census work lives in TODO_LIST.md + docs/census.md.

# Status Report — Plan Execution Session 2: Scale Tier (T5 analysis, T7–T11)

**Written:** 2026-11-10 06:58 CET · **Session scope:** continuing the
"WHOLE TODO LIST" directive against plan v2
(`docs/planning/2026-11-10_01-18_pareto-plan-v2-journal-scale-deghost.md`).
Session 1 (report 01:49) had landed T1–T4, T10, and T17-partial before
exhausting itself mid-T5. This session picked up at T5-in-flight and built
the scale tier. **Note:** session resumed after a ~5 h pause (last action
01:53, report written 06:58); background jobs completed during the gap.

## Scope line — verified vs not verified

**Verified, with commands:**

- Baseline at session start: `go build ./... && go vet ./...` green, tree
  clean at ff0a81b.
- T5 original run (launched by session 1): completed 01:54 — 3645 domains,
  workspace certain 304 / likely 134, gcp 41, errors 1625. Replay diff via
  python content-compare: **0 differences across all 3645 domains**.
- 10-verdict dig cross-check: **10/10 correct** (3× MX aspmx.l.google.com,
  2× SPF include:_spf.google.com, 1× GCP IP 34.144.248.61 ∈
  34.144.192.0/18 per live cloud.json fetch, 2× negatives, 2× disputed
  "nxdomain" — see the finding below).
- False-NXDOMAIN audit: 14/14 sampled recorded-nxdomain domains answered
  NOERROR at 9.9.9.9/1.1.1.1 with live NS/A/MX records.
- T7 50k-domain streaming smoke: EXIT=0, WALL=1682s, **MAXRSS=40MiB flat**,
  50000 jsonl lines, taxonomy tail (49525 nxdomain, 475 timeout).
- T11 G4 benchmark (`go test -run OneHundred -v`): **PASS** — 100k events:
  append 18.7s (5345/s), replay 2.0s (51035/s), db 125MiB, heap 15MiB.
- Final gate at report time: `go build ./...`, `go vet ./...`, and
  `go test ./internal/scanstore/ ./internal/cli/ -count=1 -short` all green.
- Full `buildflow` gate still blocked by BuildFlow#56 (toolchain skew,
  documented in AGENTS.md); direct gates used per that policy.

**NOT verified / invalid:**

- The T5 "clean rerun" (--resolver 9.9.9.9, job started 01:53, done during
  the pause): EXIT=0, 657s, 65MiB, but **timeout 2157** — 9.9.9.9
  rate-limited 24 workers; verdict counts (168 certain / 92 likely / 16
  gcp) are UNDERCOUNTS, not comparable to the 304/134/41 of run 1.
  **G3 (tag readiness) is NOT met by either run.**
- The before/after memory A/B (old buffered binary vs new streaming, same
  50k list): **INVALID — I killed my own benchmark.** My `pkill -f
  "/tmp/gdd scan"` (aimed at the prior session's zombie scans) matched the
  A/B subprocess too; it died mid-run (evidence: 37921 "canceled" errors in
  its stderr summary, only partial output). The new-binary 40MiB number
  stands (independent run); the old-binary comparison needs a redo.

---

## a) FULLY DONE (this session)

1. **T7 — jsonl streaming output** (daemon-committed 09ff026/e7d5e3c).
   `render.JSONLStream`: mutex-serialized incremental writer, first-error
   sticks, documented semantics (completion order, cancel = stream just
   stops, unstarted domains emit nothing). `chainOnResult` in cli chains it
   behind the event-store sink. Tests: completion-order pinned by
   slow-domain-lands-LAST assertion (a buffered renderer would print input
   order); cancel-mid-stream with explicit cancel → "canceled" error kind.
   50k smoke: 40MiB flat RSS.
2. **T8 — streaming end to end** (daemon-committed 2f0ff21). Three parts:
   (1) `input.Streamer` — line-by-line parse, lazy plain/CSV mode detection
   off the first non-empty line, skipped entries counted exactly but capped
   at 5 kept examples; `input.Parse` is now a thin wrapper (behavior
   preserved, existing tests untouched-green). (2) `domainFeeder` in
   internal/cli/domains.go — chunked (scanChunkSize=4096, test-shrinkable
   var) cross-source dedup in first-seen order; `runScan` scans chunk by
   chunk, jsonl discards results per chunk, buffered formats materialize as
   before; input totals journaled exactly when everything fit one chunk,
   0=unknown otherwise. (3) `walkJournal` batched journal walker +
   `StreamObservations`; `ReplayJournal` reimplemented as two bounded
   walks (no full event materialization); `replay --format jsonl` streams.
   Tests: args/file/stdin parity, multi-chunk scan+record+replay round-trip
   (chunk size 4), StreamObservations ≡ ReplayJournal equivalence,
   ErrNoRuns sentinel, 10k-event streaming test.
3. **T9 — journal-based --resume** (commit 3674285). `scanstore.
   LoadResumeSource` (skip-set = domains observed in a run; exact run ID or
   "latest"; validates run exists), `ScanStartedPayload.ResumedFrom`
   (additive schema field, decider mapping included — the missing decider
   mapping was caught by the test), `RunSummary.ResumedFrom`, `--resume`
   flag (usage error without --event-store; zero-remaining → clean no-op
   with stderr note). Tests: resume skips observed + links runs; resume of
   an interrupted run (started, never completed).
4. **T5 analysis** (verdict pending, but the analysis itself is done):
   replay content-diff 0/3645; 10/10 dig cross-checks correct; error tail
   readout delivered by T10's taxonomy. **Critical finding recorded below
   (§d/§e).**
5. **Zombie cleanup:** killed the prior session's two orphaned scan
   processes that were poisoning the local resolver (see finding).

## b) PARTIALLY DONE

1. **T11 — scanstore hardening.** DONE: Sink bounded retry (3 attempts,
   25/50ms backoff) via a `runRecorder` seam; loud Close error naming the
   exact dropped-observation count; `CompleteScan` on
   `context.WithoutCancel` so Ctrl-C still closes the run record; 100k
   benchmark test PASSING (numbers above). NOT DONE: the ctrl-C drain
   integration test (11.4, designed: hang-resolver + explicit cancel +
   replay-the-partial-run assertion) and the G4 verdict entry in AGENTS.md
   (11.6) — both blocked on nothing but the session pause.
2. **T5 tag-readiness (G3).** Original run: honest verdicts, dishonest
   error tail (resolver poisoning). Clean rerun: honest errors, rate-limit
   gaps (2157 timeouts undercount verdicts). Needs ONE more run with
   gentler settings (workers ~8–12, timeout 8s, calm resolver — the local
   one is calm now that the zombies are dead). Then CHANGELOG + verdict.
3. **Docs debt from this session:** no CHANGELOG entries for T7/T8/T9/T11;
   no AGENTS.md gotcha for the resolver-poisoning finding. (T17 owns the
   bundle, but the entries should have been written as I went.)

## c) NOT STARTED (this session)

T6 (release v0.1.0 — gated on G3), T12 (show/runs read-model consumers),
T13 (go-output polish), T14 (README + tap, post-tag), T15 (WithDoctorCheck),
T16 (nom progress), T17 remainder (TODO merge, FEATURES, STACK rows,
docs/status index, DENIC retry, tidy, docs-gates), T18 (audit upkeep + cqrs
re-audit), T19 remainder (DENIC research retry, crt.sh extractor; Nominet
email + SWITCH TSIG request are owner actions), T20 (lint to zero — LSP
shows ~100+ warnings; real count needs a golangci run), T21 (cmdguard-lint
CI), T22.3 (RegisterUnknownRenderer; T1 decided against `-o`, so 22.1/22.2
are resolved-as-decided-against), T23 (registry-derived enum), T24 (NXDOMAIN
pre-sweep; T9 ✓, G4 nearly ✓, T19 pending), T25 (CSV enrichment), T26
(scoped exit codes), T27 (infra long-tail + ROADMAP).

## d) TOTALLY FUCKED UP

1. **I killed my own benchmark.** `pkill -f "/tmp/gdd scan"` — meant to
   reap the prior session's zombies — also matched the before/after A/B I
   had just launched. The "before" memory number is invalid (37921 canceled
   errors prove it died mid-run). Lesson: pkill by PID, never by a pattern
   that matches my own background jobs; or launch A/B after the reaping.
2. **The T5 "clean" rerun was mis-designed.** 24 workers against a single
   public resolver (9.9.9.9) is exactly the rate-limit hammer the plan's
   own T11/G4 thinking warns about. I traded a poisoned resolver for a
   throttled one. G3 still unmet; my fault, not the network's.
3. **Python string surgery on the test import block** mangled
   scanstore_test.go (duplicate imports, broken string literal) — 3 repair
   rounds. Should have used the edit tool with exact context.
4. **multiedit silent partial failures twice.** The big runScan replacement
   reported "applied 3 of 4" with the load-bearing edit silently not
   landing; I only noticed via the attachEventStore signature mismatch.
   Also wrote a `mustTuple` placeholder hack mid-edit that had to be
   excised. Never again: verify the symbol landed; no placeholder code.
5. **First-draft domains.go had a double-read bug** (two streamers on one
   `os.File`) and a phantom error type — caught in self-review before
   build, but sloppy.
6. **Daemon commit race lost again:** T7/T8 landed as junk-message daemon
   commits (09ff026, e7d5e3c, 2f0ff21) because I batched the commit after
   both tasks. Session 1's report already said "commit immediately after
   each edit batch"; I repeated the mistake anyway. T9/T11 fared better
   (3674285 + daemon sweep of finished files).
7. **LSP dead all session** (jsonrpc connection closed; stale typecheck
   garbage persisted in diagnostics). I worked off the compiler, which was
   right, but `lsp_restart` should have been the first move.

## e) WHAT WE SHOULD IMPROVE

1. **Resolver-trust product question surfaced by T5:** a distressed local
   resolver (127.0.0.1 under concurrent load) synthesized NXDOMAIN for
   live domains; stdlib `net.Resolver` cannot distinguish an authoritative
   NXDOMAIN from a lying cache. Our invariant "NXDOMAIN is never retried"
   is right for honest resolvers and wrong for this failure mode. Candidate
   mitigations (decision needed, see §g): one verification retry for
   NXDOMAIN on bulk runs only; or keep the invariant and document
   `--resolver` as the bulk-run default (AGENTS.md already hints this).
   Either way: AGENTS.md gotcha entry is overdue.
2. Kill processes by PID; launch benchmarks after reaping, not before.
3. Validation runs need worker/resolver budgets sized to the resolver
   (24 workers = system resolver only; public resolvers want ≤ 8–12).
4. CHANGELOG line per task as definition-of-done, not a T17 cleanup item.
5. `lsp_restart` when diagnostics contradict the compiler.
6. The 100k extrapolation (2.5M domains ≈ 8 min append, ~3 GiB db, ~50 s
   replay) should be recorded in AGENTS.md as the G4 evidence.

## f) Up to 50 next things (ranked)

1. Finish T11: ctrl-C drain integration test (11.4) + G4 verdict + 100k
   numbers into AGENTS.md (11.6).
2. T5 run #3: workers 8, timeout 8s, system resolver (zombies dead) or
   1.1.1.1; then error-tail readout + replay diff + verdict (G3).
3. Redo the before/after memory A/B (old binary, BY PID protection).
4. CHANGELOG entries for T7/T8/T9/T11 + resolver-poisoning note.
5. T6: release v0.1.0 once G3 green (go-release skill, tag, both remotes,
   GitHub release, proxy check).
6. T12: `show <domain>` (observation history) + `runs` commands — de-ghost
   the read models; RunSummary.ResumedFrom now feeds `runs` output.
7. T13: go-output polish (--color, SetFooter summary, Title, AddRowChecked).
8. T15: WithDoctorCheck preflight (resolver reachability + ranges feed).
9. T16: nom live progress (TTY-only), now that streaming (T7) exists.
10. T17: merge plan-v2 delta rows into TODO_LIST (docs-gates freshness
    first), FEATURES refresh (streaming/resume now real), STACK rows,
    docs/status index in README, `go mod tidy` sweep, docs-gates green.
11. T18: audit upkeep — re-verify citations vs new HEAD, cqrs re-audit
    (premise flipped by scanstore), verified-at-commit headers (22 reports).
12. T19: retry DENIC research (was rate-limited); crt.sh fallback extractor
    script + fixture test; README bulk docs wiring.
13. T20: lint to zero (golangci run for the real list; known: err113 ×N
    incl. my new encodeJSONLLine errors, cyclop scan.Run/ScanDomain,
    wrapcheck, errcheck Fprintf, tagalign, varnamelen, exhaustive, makezero,
    nilnil, tagliatelle on json snake_case tags — several are deliberate
    style conflicts to resolve by policy, not blind fixing).
14. T21: cmdguard-lint step in ci.yml, expect 0.
15. T22.3: RegisterUnknownRenderer for the JSON DTO — delete the cli.go
    special-case (json/jsonl still route around renderResults).
16. T23: registry-derived `values:` enum + drift-proof test.
17. ~~T24: NXDOMAIN/A-existence pre-sweep (gates: T9 ✓, G4 ~✓, T19 pending).~~ superseded — the in-scan existence oracle (resolve.LookupExistence, 2026-11-13) replaces any pre-sweep
18. T25: CSV enrichment mode.
19. T26: scoped exit codes in ranges (75 fetch / 65 corrupt).
20. T27: TimeoutMiddleware, describe contract test, vendorHash.nix
    extraction, audit version-freshness guard, docs-gates cache-key
    investigation, ETag, ROADMAP parking (incl. CAA watch).
21. Owner sends Nominet application email + SWITCH TSIG request (drafts in
    docs/zone-data/, ready since session 1).
22. Push decision (see §g): 15+ local commits on both remotes unpushed.
23. Re-run `nix build` after the T7–T11 dep-neutral changes to confirm
    vendorHash still valid (go.mod untouched this session — likely fine).
24. Investigate: jsonl stream + `--only` filter currently filters lines
    client-side post-scan; document that filtered streaming is still
    full-scan (no DNS savings) or skip filter for jsonl.
25. `describe` output for scan now includes --resume — eyeball the
    generated help once (cmdguard battery).
26. Sibling session is active again (commit a9e41af "Plan the Tranco
    top-1M census") — coordinate: their census plan likely consumes my
    resume/streaming work; don't diverge TODO_LIST edits.
27. Add a feed-freshness/UA integration note once T14 touches README.
28. Bench jsonl stream throughput on the calm network (the 28 min 50k run
    was zombie-contended; expect far faster now).
29. Consider `--workers` default doc hint (8 is safe everywhere).
30. Empty-input edge for `--resume` when store has zero runs → currently
    ErrNoRuns wording could name the store path (minor UX).

## g) Questions for the owner (cannot be figured out from the repo)

1. **NXDOMAIN retry policy:** keep the "NXDOMAIN is authoritative, never
   retried" invariant and document `--resolver` as the bulk-run mitigation,
   or add a single verification retry for NXDOMAIN verdicts under a bulk
   profile? (Changes a pinned invariant — your call, not mine.)
2. **Push cadence:** ~15 local commits sit unpushed on a two-remote repo;
   plan defers pushing to the T6 tag. Push now to origin + github mirror,
   or hold everything for the release push?
3. **G3 acceptance bar:** if run #3 (gentler settings) still shows a
   residual timeout tail (say < 2%), does that count as "explained error
   tail" for the v0.1.0 tag, or do you want a specific ceiling (e.g. 0.5%)
   codified first?

---

_Session artifacts: /tmp/t5-clean.{db,json,stderr} (run 2), /tmp/t5.db +
/tmp/t5-scan.json (run 1), /tmp/t7-50k-_ (streaming smoke), invalid A/B at
/tmp/t7-50k-before.* (killed mid-run). Test gates green at report time.*
