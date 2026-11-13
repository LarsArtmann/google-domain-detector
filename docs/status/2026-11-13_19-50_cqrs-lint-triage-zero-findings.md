# cqrs-lint driven to zero: 14 findings triaged (2 real fixes, 12 justified suppressions)

**Session:** 2026-11-13 19:29 → 19:50 CET (the triage the 10-23 report
queued as §f1, executed after the owner re-pasted the 14-finding dump
with an execute mandate). Ran entirely offline against a COPY of the
journal state — no scans launched (wave B single-flight respected).

## a) DONE

1. **All 14 cqrs-lint findings dispositioned; uncached gate = 0.**
   Two REAL fixes:
   - **F031** `internal/scanstore/queries.go` — `ListRuns` passed no
     limit to `reader.Scan`, and metaengine **silently caps at 100
     rows** by default (verified in `scan_options.go` of metaengine
     v4.23.0: "Scan and ScanPage default to a limit of 100"). A journal
     with >100 runs would hide its oldest runs from `--resume`/`--run`
     selection and the supersession audit. Now explicitly
     `metaengine.WithLimit(0)` (documented "unlimited") with a comment.
   - **C023** `internal/cli/cli.go` — the run-start failure path closed
     the event store with `_ = sys.GracefulClose(...)`, swallowing
     close errors (pending events silently losable). Now reports the
     close error to stderr, mirroring the success path.
   Twelve justified `//cqrs-lint:ignore(RULE)` suppressions WITH
   rationale at each firing site: A032 (journal-surface `ResumeSource.RunID`,
   same precedent as `RunSummary`/`Replay`), B004 ×3 (cqrs-gen
   constructors are codegen overkill for internal commands built at one
   call site each), B005 + E003 (the AGENTS.md-documented deliberate
   non-fixes), C013 ×2 (view timestamps are always UTC — built from
   `event.Instant.Time()`, verified in event v4.15.2 — so no timezone
   exists to lose in storage), C015+C023 (read-only `--input` file
   close after a clean EOF; nothing buffered can be lost and a close
   failure must not abort a whole-TLD scan), F005 (schema v1 is the
   INITIAL version; upcasters arrive with the first v2), F007 (commands
   dispatch in-process, sequentially, single-flight; start/complete
   handlers are naturally idempotent — no at-least-once bus exists).

2. **Sibling-introduced finding handled.** A concurrent session added
   `internal/scanstore/cborsanitize.go` (16:42, CBOR UTF-8 repair) mid-triage.
   It became the package-level anchor for F005/F007 (findings anchor to
   the alphabetically FIRST scanstore file — my fresh delta.go
   suppression went stale within the hour) and carried an unsuppressed
   F032 at error severity. Moved the F005/F007 suppression to its
   package clause, added the same F032 import rationale events.go
   already carries. Respected, not reverted.

3. **Mechanism research** (source-level, metaengine v4.23.0 +
   cqrs-lint v4.21.0 in GOMODCACHE): suppression matching is
   line-based (same line or first non-blank line above; comma-separated
   rule IDs supported); **stale-suppression detection warns by default**
   — which is why `SupersessionFinding.RunID` deliberately got NO
   ignore comment (A032 never fires there; a speculative one would warn
   forever). gofmt normalizes doc-comment directives to the spaced
   `// cqrs-lint:` form; the parser accepts both.

4. **Docs**: CHANGELOG (Fixed: the two real fixes + zero-findings note),
   TODO_LIST (triage row closed with full disposition summary; NEW row
   for the file-size debt below), AGENTS.md (cqrs-lint gotcha: anchor
   rule + rationale-at-site law, kept under the 220-line cap).

## b) Evidence / verification state

- `go build ./...` clean; `go test -count=1 ./...` 9/9 packages green;
  `golangci-lint run internal/scanstore/... internal/cli/...` 0 issues.
- `BUILDFLOW_NO_RESULT_CACHE=1 buildflow -s cqrs-lint` → **0 findings,
  0 suppressed, no stale-suppression warnings** (twice, bracketing the
  sibling's file landing).
- `buildflow -s go-structure-linter` → 0 (after compressing my AGENTS.md
  addition; see §c1).
- `buildflow -s file-size-check` → 9 errors, **all pre-existing** (every
  flagged file was over the 350-line cap before this session: cli_test
  1763, cli.go 769, coverage_test 577, scanstore_test 587, replay 489,
  scan_test 476, ranges_test 356, render_test 357, scanstore 378). Not
  this session's damage; queued as a TODO_LIST row (splitting cli.go is
  a dedicated session). Full-mode gate fails on these 9 only.
- vulnix: 21 warnings (nix store drvs — binutils/cargo/perl CVEs);
  warning severity, gate passes. Fleet/toolchain territory, not repo code.

## c) Confessions

1. **I regressed AGENTS.md over its 220-line cap mid-session** (+13
   lines on my first gotcha attempt → 235 → excess 15, caught by
   go-structure-linter only because I ran the full gate). Compressed in
   three rounds to a 2-line gotcha. Lesson: the cap is enforced at
   ERROR severity — check the line count BEFORE adding, not after.
2. **My first F005/F007 suppression anchored to the wrong file within
   the hour**: package-level findings anchor to the alphabetically
   first package file, and the sibling added cborsanitize.go (which
   sorts before delta.go). Caught by the fresh re-run, not by me
   anticipating it. The anchor rule is now an AGENTS.md gotcha.
3. Triage discipline note: 12 of 14 findings became suppressions. That
   is the honest outcome (the codebase deliberately sits outside
   cqrs-lint's fleet defaults in exactly those spots), but the
   rationale-per-site law is what keeps it from becoming a suppress
   button — every comment names WHY, and stale detection polices them.

## d) Near-misses / environment

- BuildFlow's result cache replayed a stale finding list twice today
  (223-line AGENTS.md reported as 235, then 225 after edits) —
  `BUILDFLOW_NO_RESULT_CACHE=1` required for every verification read
  (existing gotcha, re-confirmed).
- Concurrent-session hazard is REAL and benign-ish: the sibling edited
  cborsanitize.go into existence AND compressed the gh-DNS gotcha in
  AGENTS.md while I held a stale view of that exact line (one edit
  failed cleanly, re-read, moved on). Two agents in one repo worked,
  but only because every collision was a failed no-op, not a clobber.

## e) Numbers

- Findings: 14 → 0 (plus the sibling's F032: 15 total dispositions).
- Real code fixes: 2 (queries.go, cli.go); suppression comments: 12
  sites + 2 package-clause/import directives on cborsanitize.go.
- Tests: 9/9 packages, `-count=1`; golangci 0 on touched packages;
  cqrs-lint + go-structure-linter both 0 uncached.

## f) Next tasks (harvest)

- **Split the 350-line-cap violators** (9 files, full-gate blockers) —
  TODO_LIST row added with the complete list; cli.go alone owns
  scan+replay+show wiring and needs its own session.
- `runs --format` jsonl with full run IDs (existing row, unchanged).
- Wave-B completion path (wrapper stage verification → poisonfix launch
  → wave C) — tracked in docs/census.md ledger; owner go pending.

## g) Open questions

- None new. (Prior §g items from the 10-23 report: cqrs-lint was
  confirmed the intended work order by the owner's re-paste; wave-C
  auto-launch and CI-babysitting policy remain as stated there.)
