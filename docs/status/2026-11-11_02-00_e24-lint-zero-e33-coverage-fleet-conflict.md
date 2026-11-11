# Status Report — E24 lint sweep driven to ZERO, fleet formatter conflict resolved, CLI coverage 71.6→84.2%

**Written:** 2026-11-11 02:00 CET · **Session scope:** resume from the
19-08 handoff pair. This session finished the E24 lint sweep (45
remaining findings → **0 authoritative**), resolved a fleet-internal
formatter contradiction nobody had named before, refactored the four
complexity hotspots (runScan, runReplay, domainFeeder.next,
TestQueryReadModelsRoundTrip), reversed one handoff claim that the test
suite proved wrong, and pushed the E33 coverage half from 71.6% to
84.2%. Tip `ab31ad9` CI **GREEN** on both remotes; race suite 8/8;
docs-gate 0 fail.

> **HEADLINE for the owner:** the census sibling's newest push
> `f21c845` (E16 cleanup loop) is **CI RED** (run 42934256399, failed
> in 1m48s). Not my lane — flagged, not touched. Everything below is
> otherwise green.

## a) FULLY DONE (this session)

1. **Authoritative lint count baseline:** buildflow findings JSON at
   session start = **45 findings** (the handoff's "~30" estimate was
   high; the batch had cleared more than expected). Grouped by rule and
   worked file-by-file from the JSON only — the editor LSP panel was
   again full of phantoms (stale line numbers, already-fixed findings)
   and was ignored throughout, per the standing lesson.
2. **scanstore package cleared** (7 findings): `applyDomain`
   singleCaseSwitch → if; `delta.go` `c`→`counts` + two wsl cuddle
   fixes; `views.go` dropped the no-op `omitempty` on `StartedAt`
   (byte-identical JSON, honest tag — NOT omitzero, which would have
   changed the journal-derived view contract); `replay.go` godoc-start
   fix, cyclop 13→~10 via `slices.ContainsFunc`, `errUnknownRun` wrap
   (`%w` chain preserved, message extended), `errUnsupportedStore`
   sentinel (message byte-identical), `p`→`payload`; `failedResult()`
   unparam'd (parameter was always the same constant); `queries_test.go`
   rewritten into `recordRun`/`assertLatestObservation`/`assertRunViews`
   helpers (gocognit 30 → well under threshold).
3. **The contextcheck pair resolved with one reversal:** `progress.finish`
   now takes ctx and publishes `WithoutCancel` (honest ctx flow); but
   `NewSink→record`'s "should pass context" was **WRONG** — the handoff
   called it a real finding, and it is not: `TestScanCancelMidRunDrainsEventStore`
   pins that a Ctrl-C'd scan still journals every completed observation
   and closes its run (the journal IS the --resume checkpoint). My ctx
   change broke that test (0 journaled on cancel). Reverted; the
   deliberate detach is now documented on `NewSink` itself and the call
   site carries `//nolint:contextcheck` with the invariant named.
4. **runScan complexity refactor (gocognit 38 / later cyclop 16 → within
   limits):** extracted `scanOutputOpts` (flag trio), `resolveResume` +
   `resumeSkipSet`/`resumeRunID`, `scanTargets` (feeder + first chunk +
   empty-input semantics, nil-feeder no-op for fully-observed resume),
   `attachScanRecording` (input totals + wave/store pairing),
   `newScanStream` (jsonl vs TTY vs pipe wiring), `scanAllChunks`
   (chunk loop), `renderOutput`. Each helper individually trivial.
5. **runReplay refactor (gocognit 44 → within limits):** split into
   `runReplayStreaming` (jsonl) / `runReplayBuffered` (everything else)
   / `applyDelta` (flip narrowing + tallies), plus package-local
   `runSummary`. Deleted the dead `baseline.runByID` field (unused),
   added `provenanceParts` const (mnd), errcheck'd the ruleset warning.
   Nestif 10 disappeared with the branch.
6. **domainFeeder.next (gocognit 30 → ~4):** extracted `drainStreamer`
   (per-source drain incl. close + skip tally), `foldSkipped`,
   `accept` (skip-list/dedup gate). Behavior identical, first-seen
   order preserved.
7. **The fleet golines vs tagalign contradiction — named, proven,
   resolved.** The fleet golines formatter demands padded,
   flag/help-first tag keys; the fleet tagalign default demands
   full-alphabetical; my project policy layer (E24) demanded unpadded
   alphabetical. I proved by experiment that NO file content satisfies
   golines+tagalign simultaneously (three candidate styles each failed
   one of the two). Resolution: keep the project's sorted-unpadded
   policy (documented in `.golangci.yml` with the conflict rationale),
   `//nolint:golines` on exactly the three flag structs, with pointers
   to the config comment. This is BuildFlow#56-family tooling skew —
   the fleet's own two binaries disagree with each other.
8. **varnamelen `f` policy landed** (owner-question default from the
   handoff): `f` added to ignore-names with the cmdguard-convention
   rationale (`f XxxFlags` receivers). Local renames for the rest:
   `d`→`dispatcher` (cli_test), `f`→`res` (scan_test fake).
9. **Mechanical tail cleared:** `errors.Is` for the NXDOMAIN
   short-circuit sentinel; makezero nolints ×2 with the presized/
   index-filled rationale (input-order contract — appending is the
   bug, not the fix); gochecknoglobals nolint on the `-X main.commit`
   ldflags stamp; show.go tags sorted single-spaced; fuzz seed typo
   fixed (`UDPATED`→`UPDATED`, same test value) + corpus subtests
   parallel; nonamedreturns on `next()`; unparam'd `ctx` off
   `scanTargets`; three multiline signatures (probeApexRecords,
   probeHosts, recordRun) + one wsl cuddle dance.
10. **Final E24 state, verified:** `buildflow -s golangci-lint` → **0
    findings**; `golangci-lint run ./...` → **0 issues** (before the
    coverage tests landed); build + vet + full suite green; `-race`
    full suite 8/8 green; docs-gate 0 fail (18 pre-existing warns).
    192 → 0 across the two sessions.
11. **Pushed `ab31ad9` to BOTH remotes with a real message; CI GREEN**
    (run 42890417878, 5m56s, the new `-race` CI lane included). All
    intermediate daemon-swept commits also green.
12. **E33 coverage half: 71.6% → 84.2%** via new
    `internal/cli/coverage_test.go` (14 tests): `Run` bad-flag usage
    error + unknown-command (covers Run, executeWithUsageContract,
    stdin/stdoutIsTTY without touching DNS); `loadRanges` fresh-cache
    hit (httptest-seeded cache, unreachable URL proves no-network
    path) + `--no-ranges` nil; `scanTargets` resume-no-op + empty-input
    usage error; `newScanStream` all three modes (jsonl stream, TTY
    progress wiring + finish render, pipe no-op); `nomProgress`
    lifecycle (started/done/fail/finish against a buffer); `runRefOrLatest`;
    `warnRulesetMismatch` 5-case table; `resultError`; doctor
    `checkRangesFeed` pass + fail via env-driven probes;
    `defaultRangesCache` shape.
13. **Two race-class test hardenings:** `assertLatestObservation` now
    awaits `Observations >= 2` (was asserting the moment the FIRST
    run's projection landed — a latent flake that fired once under
    suite load); `showAfterScan` retry helper added because the two
    `show`-after-scan tests race the async projection catch-up (found
    under `-coverprofile` instrumentation slowdown, passes uninstrumented).

## b) PARTIALLY DONE

1. **E33 coverage:** 84.2% vs the 90% target. Remaining zero-coverage
   is TTY/os-bound code (progress render internals, `Run` happy path
   needs DNS-free injection, `defaultRangesCache` partial) — reachable
   but diminishing returns; ~2 more tests would clear most of the gap.
2. **7 lint findings re-introduced by coverage_test.go and left
   uncommitted-then-daemon-swept:** dogsled (line 179),
   golines (file-level, one long line), nolintlint ×2 (the two
   paralleltest directive explanations — need the machine-readable
   form), wsl_v5 ×3. Committed by the daemon in `ccbc958`/`fa7d525`
   before I could fix them. NOTE: `golangci-lint` direct runs are
   CURRENTLY unusable for verification — the running `go` selected the
   downloaded go1.27.2 toolchain whose export data the go1.27.0-built
   linter cannot read ("export data version 5 > maximum 4", 38 phantom
   typecheck errors while `go build` is green). Verification must go
   through buildflow's count (its env handling worked earlier tonight).
3. **TODO_LIST / AGENTS rows for this session:** not yet struck/written
   (E24 row, coverage row, the fleet-conflict gotcha, the NewSink
   invariant note).
4. **This status report itself** is the session's doc deliverable; the
   daemon will sweep it.

## c) NOT STARTED

1. **E25 CI lint step** — now UNBLOCKED (E24 = 0), but `ci.yml` is the
   sibling's E25 file; coordination needed before adding a golangci-lint
   job (and the toolchain-skew phantom above must be understood or the
   CI lint step will go red for the wrong reason).
2. **E22 spot-check** of the sibling's audit-header claims (sample a
   few of the 22 pinned reports).
3. **Tap README wording reconcile** ("planned post-tag" vs sibling's
   "tap done" scope line).
4. **reports/ durability policy** (gitignored `reports/` referenced by
   release notes/CHANGELOG — fresh clones can't see them).
5. **Consumer propagation sweep** for v0.1.0.
6. **Fleet-config upstream report** for the golines/tagalign binary
   contradiction (their repo, not this one).

## d) TOTALLY FUCKED UP (owned, with lessons)

1. **The `return scanErr` slip in my runScan rewrite.** Mid-refactor I
   returned the scan error where the original returned the render
   error — would have silently swallowed report-rendering failures.
   Caught by my own re-read BEFORE commit; `rendered` restored. Lesson:
   after extracting helpers, re-diff every `return` in the rewritten
   function against the original's error-flow.
2. **A stale-read edit deleted an assertion body.** Converting
   TestShowJSONCarriesEvidence, my old_string/new_string pair cut the
   assertion + closing brace, leaving `byDomain := ...` dangling into
   the next func. Caught by build within a minute; restored with the
   correct showAfterScan conversion. Lesson: after ANY "file modified
   since read" rejection, re-view the WHOLE target region, not just the
   first lines.
3. **Trusted the handoff over the test suite.** The 19-08 report called
   the contextcheck `NewSink→record` finding "REAL — should pass ctx
   into the Observe path"; I implemented it, `TestScanCancelMidRunDrainsEventStore`
   failed with "partial run must journal both domains, got 0", and the
   invariant comment at the test's head told me why. Cost: one broken
   build cycle. Lesson: handoff claims about INTENT need the pinned
   test read first — the test is the contract, the prose is a guess.
4. **A non-compiling draft in the new coverage tests** — I wrote
   `warnRulesetMismatch` calls as if it returned values (it's void),
   plus a stray echoWriter helper. Caught by my own read-back of the
   written file before any build ran. Lesson held from last session:
   read back new files immediately after write.
5. **~30 minutes spent proving the golines/tagalign conflict** via
   three successive write-and-lint experiments before running the
   decisive single-package golangci check. The decisive experiment
   should have been first. Lesson: when two tools disagree, run both
   against the SAME file in ONE command before theorizing about their
   canonical forms.
6. **The daemon swept this session's work into eight junk commits** and
   beat me to the commit twice even when I had the message drafted. I
   amended exactly one of them with a real message (`47335fe`); the
   rest stand as `chore: auto-commit …`. History readability keeps
   degrading session over session (now a standing §e item, and a §g
   question).
7. **A heredoc experiment truncated show.go** (I overwrote it with a
   25-line test stub). Restored from git within the same minute — no
   loss, but a write-tool discipline slip: heredocs are for NEW files,
   never for overwriting existing source.

## e) WHAT WE SHOULD IMPROVE

1. **Commit after every green batch** — the daemon raced me all
   session; explicit-path `git add` immediately after each multiedit
   narrows the window to seconds, not minutes.
2. **Verify handoff "real finding" claims against pinned tests before
   implementing** (NewSink lesson, now encoded in code comments so the
   next session doesn't re-litigate it).
3. **The fleet binaries need an upstream fix**: golines and tagalign
   (fleet defaults) emit contradictory canonical forms for the same
   struct. Until the fleet rebuilds them in agreement, every
   tag-bearing struct in every LarsArtmann repo will need this same
   nolint dance. Report to the crush-config/buildflow repo with the
   show.go reproduction.
4. **golangci-lint direct runs need a pinned toolchain** (GOTOOLCHAIN or
   a rebuilt binary) or they report phantom typecheck storms exactly
   when you need them most. buildflow's wrapped run handled env
   correctly all session; raw runs did not.
5. **Projection-catchup races are a pattern**: three tests now await
   specific projected state rather than "first response". A shared
   helper (or scanstore-side read-your-writes guarantee) would retire
   the whole class. Worth a deliberate design pass, not another patch.
6. **Daemon-swept history**: the one-amend-per-junk-commit policy works
   but is reactive; consider a session-end squash of `chore:` runs
   (owner call — history rewrite on shared master).

## f) Up to 50 next things (ranked)

**My lane (finish what I started):**
1. Fix the 7 lint findings in coverage_test.go; verify via
   `buildflow -s golangci-lint` (raw golangci is phantom-poisoned right
   now).
2. Re-run the full `-race` suite (coverage tests touched scanstore
   tests + env-mutating doctor tests landed after the last green).
3. Push the fixed state; confirm CI green tip again.
4. CLI coverage 84.2→90%: Run happy path via injected deps; progress
   render branch for multi-activity frames.
5. TODO_LIST: strike the E24 row with the 0-count evidence; strike the
   E33-coverage row at 90% (or annotate at 84.2% if the last 6% is
   judged not worth it).
6. AGENTS.md gotchas: the fleet golines/tagalign contradiction + where
   the three nolints live; the NewSink detached-context invariant (so
   nobody "fixes" it again); golangci toolchain-skew phantom class.
7. Write the fleet-config upstream report (show.go reproduction) for
   the golines/tagalign binary disagreement.
8. E22 spot-check: sample 5 of the sibling's 22 audit-report headers,
   verify pins point at real commits.
9. Reconcile tap README wording vs the sibling's scope line.
10. reports/ durability: draft the negation-pattern or docs/-move
    proposal for the owner (§g3).

**Coordination lanes (sibling's work — verify/flag only):**
11. Sibling's RED CI on f21c845 (E16 cleanup loop) — surface to them,
    don't touch their commit.
12. E25 CI lint step: propose the golangci-lint job now that E24=0, but
    only after the toolchain-skew phantom has a pin or a rebuilt binary.
13. Describe contract test (E31.2) — unblocked since the tree compiles.

**Census lane (sibling's — listed for completeness, OFF-LIMITS to me):**
14. Their E16 loop landed (f21c845) but CI is red — fix belongs to them.
15. Sustained-w8 Quad9 soak before wave A launch.
16. G6 pilot (5k tranco1m-a slice through census-gate).
17. Wave A/B/C execution (~7h/27h/34h) → E17 census report.
18. E10 trickle round 3 (113 timeout stragglers) + findings README.
19. servfail-stable semantics into census-gate.sh + docs/census.md.
20. E32 daily delta timer / lead scoring.
21. scripts/audit-freshness.sh (E31.4 comparing script).
22. crtsh-extract fixtures: committed-corpus variant.

**Quality/infra backlog (either lane, low urgency):**
23. E28 --pre-filter.
24. .ch SWITCH AXFR pull.
25. IPv6 resolver behavior check.
26. TXT truncation investigation.
27. asciinema demo.
28. GoReleaser binaries.
29. Consumer propagation sweep for v0.1.0.
30. TimeoutMiddleware (cqrs lane).
31. Read-your-writes projection guarantee design pass (e5 above).
32. Session-end junk-commit squash policy decision (owner).
33. E31.4 freshness script (if the sibling doesn't take it).
34. docs/DOMAIN_LANGUAGE.md harvest (census terms are accreting fast —
    wave, flip, servfail-stable, coverage bar all need one glossary).
35. FEATURES.md refresh post-v0.1.0 (replay --since, --resume, --wave,
    show are undocumented there).
36. CHANGELOG entry for the lint-zero/refactor batch (internal, one
    line).
37. Consider `gh run list` session-start habit now includes checking
    the sibling's tips too (this session caught their red CI at 02:00).
38-50. Reserved: everything else waits for the owner's §g answers and
    the sibling's census progress; no invented work.

## g) Questions for the owner (cannot self-answer)

1. **Tag style ruling:** keep my resolution (project sorted-unpadded
   tags + 3 documented `nolint:golines` on the flag structs, fleet
   conflict documented in `.golangci.yml`) — or switch to fleet-canonical
   padded tags and delete the project tagalign policy layer (which
   partially reverts the E24 sibling's aesthetic choice)? Both are
   zero-findings after the nolints; it's a policy-ownership call, not a
   technical one.
2. **BuildFlow#56 binary rebuild:** the go1.27.0-built buildflow binary
   now blocks THREE things (pre-commit hook, in-format lint steps, and
   as of tonight raw golangci runs phantom-typecheck against the
   downloaded 1.27.2 toolchain). Mine it now, or keep it parked?
3. **Daemon junk-commit history:** eight more `chore: auto-commit`
   commits landed this session (two of them carry real work with no
   message). Acceptable forever, or do you want a session-end squash /
   rewrite policy (shared master, both remotes — needs your explicit
   OK)?
