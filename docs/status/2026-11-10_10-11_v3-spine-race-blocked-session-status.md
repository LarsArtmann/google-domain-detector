# Status Report — v3 Spine Execution: plan shipped, truth debt paid, release blocked by a race

**Written:** 2026-11-10 10:11 CET · **Session scope:** the "READ → PLAN →
EXECUTE" directive against plan v3
(`docs/planning/2026-11-10_09-21_unified-execution-v3-ship-then-census.md`,
written this session). Session started ~09:15 from a tree where the
sibling sessions had landed T12 (show/runs) and T13 (--color, mid-flight
at session start, daemon-landed during it). This report covers ONLY this
session's work.

## Scope line — verified vs not verified

**Verified, with commands:**

- Plan v3 graph compiles: `mmdc 12.0.0` → 232 KiB SVG, rc=0. Committed
  0c76abd, pushed to origin AND github.
- E3 ctrl-C drain test: PASSED (plain and under `-race` in isolation,
  4/4 runs).
- CHANGELOG/AGENTS truth debt committed (58b914a): T7–T13 entries; G4
  verdict with 100k numbers; resolver-poisoning gotcha. Every claim
  spot-verified against the live binary (`--color`, `--no-color`,
  `--resume`, `--wave`, `show`, `runs` all present in `--help`).
- docs-gates: 5/5 PASS after the E5 harvest (was: freshness DRIFT 8 items
  - audit-all DRIFT 12 items; both cleared). Committed f61d8f5, pushed.
- Release pre-checks: go.mod clean (no replace, no pseudo-versions, no
  prior tags); `gh api` works despite a scary `gh auth status` warning;
  CI workflow exists (ci.yml).
- Full gates for release: `go mod tidy` + `go mod verify` green;
  `nix build` GREEN (rc=0, vendorHash valid); `go build`/`go vet` green.
- E1 replication run (workers 8, timeout 8s, calm system resolver):
  **best run yet** — certain 337 / likely 144 / gcp 74, errors 732 of
  3645 (20 %; nxdomain 596, timeout just 24, servfail 112), MAXRSS
  65 MiB, recorded to /tmp/e1-run3.db.

**NOT verified / blocked:**

- **The full-suite `-race` gate FAILED twice** on
  `TestScanCancelMidRunDrainsEventStore` (my new E3 test) — but passes
  4/4 under `-race` in isolation. Timing-sensitive under parallel suite
  load, or a real race that only manifests with contention. **The v0.1.0
  tag is BLOCKED on this** (go-release rule: never tag on a red gate).
  The race report itself was NOT captured yet (the failing runs' output
  scrolled past; isolation reruns pass).
- E1's D3 arithmetic: unknown-class (timeout 24 + servfail 112) = 3.7 %,
  above the 2 % bar — though timeouts collapsed 34× vs run 2 and G3 was
  already PASSed by the sibling's 07-46 run; this run is replication, not
  the gate.

---

## a) FULLY DONE (this session)

1. **Plan v3 written, verified, pushed** (0c76abd). Merges plan v2's
   T-series and the census plan's C-series under E-numbering (35 L1
   30–100 min tasks, ~150 L2 micro-tasks ≤ 12 min, coverage check mapping
   every open TODO row, execution graph, gates G3–G7, owner defaults
   D1–D3). Both remotes updated.
2. **E3 — ctrl-C drain formality** (58b914a): integration test proving a
   canceled scan records both observations, completes the run record,
   and the partial run replays (a valid `--resume` checkpoint). AGENTS.md
   G4 verdict recorded → **G4 formally CLOSED** (the last formality).
3. **E4 — CHANGELOG/AGENTS truth debt** (58b914a): entries for streaming
   jsonl, chunked input, streaming replay, --resume, sink retry, show/
   runs, --color; resolver-poisoning gotcha; architecture-map rows for
   domains.go/show.go.
4. **E5 — TODO truth pass** (f61d8f5): 15 rows struck/redirected
   (format-widen, atomic-write, -o decision, T4/T5 hardening+validation,
   WithCLICommit/WithExample, T20, memory row reworded); DENIC closed as
   RESOLVED-AS-IMPOSSIBLE; SWITCH updated to the verified public-TSIG
   AXFR path; three harvest blocks (07-46's 8 items, 06-58's 2, the
   01-12/06-09 back corpus' 8). docs-gates 5/5 green. Pushed.
5. **E7 verified-alive (adoption, not build):** the census session's
   unbound is running and resolving on 127.0.0.1:5353 (dig rc=0);
   config + log live in reports/census/unbound/. C05's remaining work is
   latency notes + lifecycle script.
6. **E1 replication run executed and recorded** (numbers above; artifacts
   /tmp/e1-run3.*; NOT yet archived into reports/ — see §b).

## b) PARTIALLY DONE

1. **E2 — release v0.1.0 (BLOCKED mid-flight).** Done: go-release skill
   loaded and followed; pre-checks clean; CHANGELOG 0.1.0 section cut
   (daemon-swept into e712a1f, unpushed); release notes drafted
   (/tmp/v0.1.0-release-notes.md); `gh api` probed OK; nix build green;
   tidy+verify green. Blocked: the full-suite race gate failure (§d).
   Remaining after unblock: re-run race gate green ×3, commit/push,
   annotated tag v0.1.0, push master+tag both remotes, GitHub release
   (--prerelease, 0.x), proxy/pkg.go.dev check, `go get` pull-check,
   `nix run .#release` wiring check, watch first CI run.
2. **E1 analysis half-done:** counts + error-tail readout done; replay
   diff, dig cross-checks, D3 verdict note, and archiving artifacts to
   reports/2026-11-10-t5-validation/ not done (superseded in urgency by
   the release; the sibling's G3-PASS run already carries the gate).
3. **E6 — memory A/B redo:** not started this session (planned after E1;
   E1 finished during the report window; /tmp list survives at
   /tmp/e1-domains.txt; old binary at /tmp/gdd-now is the CURRENT
   streaming build — the true "before" binary (/tmp/gdd, buffered jsonl)
   may need a rebuild from a pre-T7 commit for an honest comparison).

## c) NOT STARTED (this session)

E8 (pilot harness), E9–E18 (census lane: throughput bench, legacy
unknowns, schema-stamp finish, manifests, waves, report, delta query),
E19 (sibling-lane UX completion: footer/Title/AddRowChecked gaps, doctor,
nom), E20 (README + tap), E21 (docs bundle: FEATURES, STACK audit, status
index, tidy sweep — tidy itself ran green during release prep), E22
(audit upkeep + cqrs re-audit + verified-at-commit headers), E23 (crt.sh
extractor; DENIC research now resolved-impossible, so this shrank), E24
(lint zero), E25 (CI steps), E26/E27 (renderer/enum, sibling lane), E28
(pre-sweep; .ch zone pull row now harvested), E29–E34 (census-adjacent +
infra + owner actions). G5 sibling-lane tasks deliberately untouched.

## d) TOTALLY FUCKED UP

1. **Declared E3 "green" on a plain + isolation-race pass, then the
   FULL-suite race gate failed on it twice.** I verified the new test the
   cheap way (single -run invocation) instead of the gate way (full
   suite under -race) before writing "G4 formally CLOSED" into AGENTS.md
   and heading to tag. The test is timing-sensitive (100 ms AfterFunc
   cancel): under suite load it can hit a different interleaving. Root
   cause NOT yet diagnosed — could be a flaky test OR a real race in the
   cancel/drain path. Until diagnosed: G4-closure language in AGENTS is
   premature and the tag stays blocked. This is the session's central
   failure: I let release momentum outrun verification depth.
2. **Failed to capture the race report when it happened.** Two failing
   runs printed the failure and I piped through filters that ate the
   DATA RACE block; my follow-up isolation reruns pass, so the evidence
   is gone and reproduction now requires suite-load conditions. Next
   attempt must tee full output to a file.
3. **Repeated the edit-tool mtime trap three more times** (AGENTS,
   CHANGELOG, TODO_LIST): edited after bash-read instead of view-read,
   got rejected, burned round trips. I know the rule; I keep tripping it
   under speed.
4. **CHANGELOG 0.1.0 cut was left uncommitted** long enough for the
   daemon to sweep it (e712a1f, junk message). The release commit was
   supposed to carry it deliberately. Same daemon-race lesson, third
   session in a row.

## e) WHAT WE SHOULD IMPROVE

1. **New-test verification protocol:** a concurrency-relevant test is
   only "green" after the FULL suite under -race passes at least twice —
   isolation passes prove nothing about interleaving. Encode in AGENTS
   testing notes.
2. **Always tee gate output to a file** (`| tee /tmp/gate.log`): failing
   gates are evidence; filtered console output destroys it.
3. **Flaky-vs-real triage procedure:** run the failing test with
   `-count=20 -race` inside a suite-load context (e.g. `-run 'TestCli'
   -parallel`), and stress the specific interleaving (cancel earlier/
   later: 10/50/200 ms) before concluding anything.
4. Commit release-prep files immediately; the daemon has now eaten two
   deliberate commits this session (CHANGELOG cut + earlier T7/T8).
5. The 100 ms cancel hook in the test is fragile by construction —
   replace wall-clock AfterFunc with a deterministic trigger (cancel from
   OnResult after the first result lands).

## f) Up to 50 next things (ranked)

1. Reproduce the race: full-suite `go test -race -count=1 ./... | tee
   /tmp/race.log`; if green, loop until red (it failed 2/2 before).
2. Extract the DATA RACE stacks; classify flaky-test vs real race.
3. Fix accordingly (deterministic cancel trigger first suspect).
4. Re-run full race gate ×3 green; only then re-declare G4 closed in
   AGENTS (adjust language if needed).
5. Resume E2 exactly where it stopped: push e712a1f, tag v0.1.0
   (annotated), push tag to both remotes, GitHub release (--prerelease),
   proxy + pkg.go.dev checks, `go get` pull-check, `nix run .#release`
   check, watch first CI run (harvested TODO row).
6. Archive E1 run #3 artifacts to reports/2026-11-10-t5-validation/
   (run3 db/json/stderr) + one-line replication note in the 07-46 report
   folder README (if any) or a new small note.
7. E1 leftover analysis: replay-diff run #3, 10 dig spot-checks, D3
   verdict note (3.7 % unknown-class — decide whether servfail belongs
   in "explained" per D3 wording; likely amend D3 to name servfail as
   explained-retryable).
8. E6 memory A/B: rebuild pre-T7 binary from git (commit 4235e02^) for
   the honest "before", PID-safe launch, record table.
9. E8 pilot harness (slice + error gate + preflight doc) — unbound
   already alive.
10. E9 throughput bench via unbound (workers sweep) → wave params.
11. E10 recover 7 204 legacy unknowns via unbound + --resume.
12. E11 finish C02 stamping (sibling started --wave; coordinate G5).
13. E12 wave manifests; then E13–E15 waves A/B/C behind G6 gates.
14. E16 cleanup loops; E17 census report; E18 delta query.
15. E19 sibling-lane UX completion (verify their footer/Title/AddRowChecked
    state first — G5 re-check).
16. E20 README (fix JSON array claim, completion-order note, install,
    quickstart incl. --event-store/--resume/show/runs/replay, asciinema,
    feed-freshness/UA note) + tap post-tag.
17. E21 docs bundle (FEATURES create/refresh, STACK audit, docs/status
    index, docs-gates).
18. E22 audit upkeep (cqrs re-audit premise-flipped, citations vs HEAD,
    verified-at-commit headers ×22, re-screenshot).
19. E23 crt.sh extractor + fixture test + README bulk docs (DENIC half
    resolved-impossible already).
20. .ch zone pull via public SWITCH TSIG (harvested row; once/24h limit;
    feeds E28).
21. E24 lint zero (after G5 lane lands; policy calls on tagliatelle
    snake_case + err113 style).
22. E25 CI: cmdguard-lint + golangci + govulncheck.
23. E26 RegisterUnknownRenderer; E27 registry-derived enum.
24. E28 pre-sweep (gated on E23/.ch zone); E30 scoped exit codes.
25. E31 infra long-tail (TimeoutMiddleware, describe contract test,
    vendorHash.nix, freshness guard, docs-gates cache-key probe, ETag,
    ErrNoRuns path-in-error, IPv6 --resolver check, ROADMAP.md parking
    incl. CAA watch + M365 fingerprints + registrant dedup rows).
26. E32 census value-adds (daily delta timer, journal-only stats, lead
    scoring); E33 coverage+fuzz; E34 BuildFlow#56 fleet fix (approval);
    E35 owner: Nominet email + GPG signing policy decision.
27. Commit protocol fix: `tee` gate logs, immediate commits, view-before-
    edit discipline.

## g) Questions for the owner (cannot be figured out from the repo)

1. **D3 wording:** is SERVFAIL an "explained" class like NXDOMAIN (it is
   a server-side failure, already retried once), or does it count toward
   the 2 % unknown budget? Run #3 lands at 3.7 % unknown-class with
   servfail included, 0.7 % without — the tag gate currently rests on the
   sibling's G3-PASS run, but the bar's wording decides whether future
   runs "pass" honestly.
2. **GitHub Release despite `gh auth status` showing a failed login:**
   `gh api` works, but release creation may hit scopes/auth walls mid-
   flight. If it fails: leave the tag + pushed notes as the release and
   you create the GitHub Release object manually, or should I stop and
   fix gh auth first (needs your credentials)?
3. **Race-gate strictness for the tag:** if the race failure proves to be
   a flaky TEST (deterministic-cancel rewrite fixes it) with no product
   race, do you want the v0.1.0 tag on that fixed commit only, or also a
   `-count=5` full-suite race run as a standing pre-tag ritual (adds
   ~5 min per release)?

---

_Artifacts this session: plan v3 (0c76abd), truth-debt commits (58b914a,
f61d8f5), daemon-swept CHANGELOG cut (e712a1f), release notes
(/tmp/v0.1.0-release-notes.md), E1 run #3 (/tmp/e1-run3._), race failure
evidence LOST (see §d.2).*
