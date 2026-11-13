> **Live remainder:** every item resolved as of the 2026-11-13 docs-health pass (docs-gates green); open census work lives in TODO_LIST.md + docs/census.md.

# Session status — E6 A/B executed, release verifications closed, E24 lint sweep 192→~30, mid-refactor

> **ADDENDUM (19:15, post-write):** the census sibling's 19-04 report
> landed while this one was being written — it supersedes several §c/§f
> entries here: **E25 CI hardening is DONE by them** (`-timeout 20m` in
> ci.yml + nightly `soak.yml`, dispatch-verified green), as are E7
> finish, E8/E9/E10/E12, E18 delta query, E23 crt.sh extractor, E33
> input fuzz (FuzzParse 30s campaign), tap, E22 headers, and the TODO
> truth pass. They also observed this session's `domains.go` break
> ("tree red under sibling edit", §d1 here) — fixed 19:07, committed
> green at 03e0682. The daemon swept the E24 batch into junk-message
> commits (`e4ba575`, `03e0682`) — content verified intact via
> `git show --stat`; the batch landed mixed with sibling files
> (`fuzz_test.go`, their report). Read their 19-04 report BEFORE acting
> on §f below — items 14-18, 20-24, 28, 31 are done or moot.

**2026-11-10 19:08 CET · @ local tree (E6 commit `64c6b80` pushed; E24 batch uncommitted, build/vet/full-test suite GREEN) · plan v3 lanes 🅐+🅑**

Resumed from the 13-27/13-28 handoff pair. This session: reconciled the
sibling's newer report (which superseded the handoff's CI plan), closed
every open release-verification item, EXECUTED the staged E6 memory A/B,
and drove the E24 lint sweep from 192 findings to ~30 via a policy layer
plus ~60 mechanical/sentinel fixes. One hand-rename break happened and
was fixed before this report (build green at write time).

## a) FULLY DONE (this session)

1. **Orientation + superseded-plan discovery.** Sibling's 13-28 report
   revealed the CI timeout was already root-fixed (100k soak now opt-in
   via `GDD_SOAK=1`, 404dfed) — my handoff's `-timeout 20m` plan was
   obsolete. `gh run list`: CI green on every commit since 404dfed
   (4 runs, tip `61c090a` 29s). All E6/E7 /tmp artifacts still alive.
2. **Release known-issue note LIVE.** `gh release edit v0.1.0` appended
   "Known issues (non-blocking)": tag commit 9308d97's CI red = soak
   timeout panic on 2-core runners, fixed on master, no artifact impact.
   Live notes verified to carry the corrected (retracted-claim-free) body.
3. **`go get @v0.1.0` pull-check PASS** (DNS cooperated): scratch module,
   `go get` + tidy + `go run …/cmd/google-domain-detector@v0.1.0
   version` compiles and runs from the module proxy; `go list -m
   -versions` shows v0.1.0. Version prints `dev` without ldflags —
   expected (documented stamping path).
4. **pkg.go.dev propagation PASS**: page live at v0.1.0 — full README,
   MIT license, internal directories rendered.
5. **E6 memory A/B EXECUTED** (sibling staged it, tmp-reaper was eating
   the runway): built the pre-T7 binary from the `/tmp/gdd-pret7`
   worktree (`4ce2c84`) and current from `61c090a`; 50k `.test` domains
   through dedicated unbound `127.0.0.1:5353`, `-j 2`, `--no-ranges`,
   VmHWM polled at 50 ms (`rssrun.sh`).
   **Result:** pre-streaming buffered json **28 080 → 41 624 KB**
   (3 645 → 50 000 domains; ~290 B/domain linear ≈ 700 MiB at 2.5M
   scale); current streaming jsonl **32 136 → 32 064 KB — FLAT** (its
   +4 MiB vs old-round-1 is newer-deps baseline, not per-domain).
   CHANGELOG's "~40 MiB flat" claim stands (actual: ~32 MiB).
6. **E6 bookkeeping closed:** results table +
   methodology in `reports/2026-11-10-e6-memory-ab/README.md`; round-1
   AND round-2 artifacts + `rssrun.sh` + binary SHA256 provenance
   archived from /tmp into `reports/` (reaper-proof; note: `reports/`
   is buildflow-gitignored — local durability only, policy gap left
   open deliberately); `/tmp/gdd-pret7` worktree removed.
7. **TODO_LIST strikes with evidence:** E6 row, go-get row, pkg.go.dev
   row, worktree-removal row, and the `gh run list` row — the latter
   adopted into project AGENTS.md Gotchas as a session-start habit
   ("the v0.1.0 tag sat on red CI for hours unnoticed").
8. **Commit `64c6b80` pushed to BOTH remotes** (E6 + TODO/AGENTS doc
   updates) with attribution trailers, pre-commit bypass documented
   (BuildFlow#56).
9. **E24 policy layer landed in `.golangci.yml`** (project-owned
   exceptions, commented): tagliatelle `json: snake` (the v0.1.0 JSON
   contract is FROZEN snake_case — renaming fields would break every
   published consumer), wrapcheck ignore-regexes for first-party org
   SDKs + `(*net.Resolver)` (adapter passes stdlib errors through by
   design; scan classifies families), ireturn allow `Resolver`
   (interface seam), test exclusions for err113 + paralleltest on
   doctor_test (global-swapping tests, deliberately serial).
   **Verified: 192 → 131 findings.**
10. **E24 code fixes, ~60 findings cleared across 12 files** (build +
    vet + full suite green at the 19:08 checkpoint):
    - `ranges`: `ErrNoPrefixes`/`ErrBadStatus` sentinels (Load's 75/65
      family still wraps them), `defaultFetchTimeout`,
      `cacheDirPerm`, `cacheFilePerm` consts; feed struct nolint for
      Google cloud.json wire format.
    - `domain`: 5 parse sentinels (`ErrEmptyDomain`,
      `ErrNotADomain`, `ErrEmptyLabel`, `ErrLabelTooLong`,
      `ErrBadLabelChars`), contextual detail now wraps them with %w;
      `s` → `normalized`.
    - `classify`: `Tally(res)` rename; error-kind switch gains explicit
      `ErrorNone` case with fallthrough (behavior-preserving);
      `IsTransient` enumerates all kinds explicitly.
    - `input`: `ErrNoValidDomains` sentinel (message byte-identical via
      `%w%s` hint trick), `Skipped()` unnamed returns, `stream` rename,
      headerKeywords nolint.
    - `cmd/main.go`: ldflags version-stamp vars nolint (standard idiom).
    - `render`: `ErrUnknownFilter` + `errNotResults` sentinels,
      `FilterAll` explicit case, `dst` rename, `init()` nolint
      (load-bearing renderer overwrite), `expectedFormatCount` const,
      `sort.Slice` → `slices.Sort` (modernize).
    - `cli/errors.go` NEW: six shared sentinels (event-store-required,
      no-runs, no-observations, resume-needs-store, wave-needs-store).
    - `cli.go`: all 9 flagged struct tags rewritten sorted + unpadded
      (tagalign satisfied, lines shortened), 8× errcheck `_, _ =`,
      resume/wave sentinels, exhaustive nolint on the format switch
      (registry-default routing is the design), nilnil nolint on
      `loadRanges`' deliberate `--no-ranges` (nil, nil).
    - `runs.go`: `errNoRuns` wrap with store path, gosmopolitan nolint
      (deliberate operator-local timestamps), perfsprint strconv
      concat, `shortRunIDLength` const, errcheck.
    - `show.go`: sentinel sites, `dom` rename, errcheck.
    - `replay.go`: tags, `errEventStoreRequired`, runSummaryLine
      errcheck, and the ErrNoRuns/ErrUnknownRun handlers now **wrap the
      scanstore sentinels with the store path** (`"%s: %w"`) instead of
      replacing them — preserves errors.Is chains AND finally names the
      store path (the parked E31 intent, done as a side effect).
    - `doctor.go`: `dnsProbeBudget`/`feedProbeBudget` consts (already
      used `errors.AsType` + %w — LSP was stale).
    - `progress.go`: `progressMaxActivities`/`progressRedrawInterval`
      consts, new static `resultError` type replacing per-event
      `errors.New(r.Err)`, 5× `_ =` OnEvent.
    - `domains.go`: `feeder`/`streamer`/`dom` renames (completed after
      the break, see §d1).
11. **`buildflow format` run** (formatter delegation): golines/gofumpt/
    goimports/dprint normalized the repo — long code lines wrapped,
    cli struct tags realigned, docs/status markdown normalized. Verified
    gofmt-clean + build + cli/render tests after.
12. **Refactors landed for 3 of 4 complexity findings:** `scan.Run`
    extracted `resolveWorkers` + `fillUnrunResults` (14 → within max),
    `ScanDomain` extracted `probeApexRecords` (NXDOMAIN short-circuit
    via explicit `errShortCircuit`) + `probeHosts` (15 → within max),
    `r` → `result` rename. scan/scan/cli/render tests green.

## b) PARTIALLY DONE

- **E24 overall:** last authoritative count 131 (post-policy, pre-batch);
  the batch above cleared the majority of the remainder — final
  authoritative count NOT yet re-measured after the batch. Remaining
  known items: scanstore package (contextcheck `NewSink->record` ctx
  pass-through, gocritic singleCaseSwitch, godoclint doc-start, cyclop
  `LoadResumeSource` 13, views omitempty modernize, unparam,
  interface-wrapcheck), `runReplay` cyclop 15, gocognit ×3 (`runScan`
  38, `domainFeeder.next` 30, `TestQueryReadModelsRoundTrip` 30),
  wsl_v5 ×7, embeddedstructfieldcheck, makezero ×2, varnamelen `f`
  params ×5 (policy decision pending, §g1), one final `buildflow
  format` pass.
- **Lane discipline held:** scanstore edits deliberately deferred — the
  census sibling went ACTIVE mid-session (`scripts/unbound.conf` +
  `scripts/unbound.sh` appeared 14:54-14:55, untracked, untouched by
  me). Their raw E7 data IS archived by me into
  `reports/census/unbound/raw/` (docs-lane item 11, no NOTES.md
  authorship — that's theirs).

## c) NOT STARTED

- E25 CI hardening: golangci-lint + govulncheck steps, nightly soak
  scheduled workflow, `-timeout 15m` belt, GOTOOLCHAIN pin (note:
  buildflow format surfaced govalid/govulncheck/golangci-in-format
  failures — all the documented BuildFlow#56 stale-toolchain class,
  not repo defects).
- E22 audit upkeep (verified-at headers ×22, cqrs re-audit).
- E33 CLI coverage 69→90% + input-parser fuzz.
- Homebrew tap (README promises it post-tag).
- Census lane (E7 write-up/lifecycle script are the sibling's, now
  visibly in `scripts/`).
- Consumer propagation sweep (likely no external consumers yet).
- varnamelen `f` policy encoding in config (§g1).
- Durability policy for gitignored `reports/` (release notes + CHANGELOG
  reference `reports/2026-11-10-e1-validation/README.md`, which a fresh
  clone will not have).

## d) TOTALLY FUCKED UP (owned, with lessons)

1. **The hand-rename break.** Renaming `d` → `dom` in `domainFeeder.next`
   via multiedit renamed the declaration and missed the 5 later uses —
   build RED, uncommitted, daemon-sweep risk. Fixed (19:07) before this
   report; suite green. **Lesson: local renames go through
   `lsp_rename`, or declaration + ALL uses land in ONE multiedit.** The
   scan.go `probeHosts` first pass had the same class of slip (invented
   `result.appendLookupErr`, dropped error aggregation) — caught by
   self-review before build, fixed with `firstErr` return-flow.
2. **Formatted mid-edit.** Ran `buildflow format` while hand-edits were
   still in flight → it reflowed files I was editing → three
   "file modified since read" rejections, one failed multiedit, several
   forced re-reads. **Lesson: complete the edit batch, then format.**
3. **Trusted stale LSP diagnostics way too long.** The editor's
   golangci_lint_ls view carried pre-edit line numbers and
   already-fixed findings (probeDNS "unused" while wired,
   "modernize errors.As" on code already using AsType, err113 quoting
   a %v that is now %w). I burned several view cycles on phantoms
   before switching to driving exclusively from buildflow's findings
   JSON. **Lesson: the findings JSON is the only authority; refresh
   after every batch.**
4. **First wrapcheck regex was wrong** (`go-output/` with trailing
   slash — missed go-output root-package sigs, 4 stragglers). Caught by
   re-diffing the count (131 not 127).
5. **Nearly repeated the PIPESTATUS trap** (`go build … | head` then
   `echo $?` = head's exit). Caught it mid-command and switched to
   redirection + explicit `$?`.
6. **Sent the IsTransient edit to counts.go** (it lives in errors.go) —
   multiedit caught it as no-match; moved to the right file.
7. **Initial plan over-complied:** was about to enumerate all 14
   output.Format cases to satisfy exhaustive; stepped back — the
   default-routing IS the design, documented nolint is the honest fix.
   Same for nilnil (`--no-ranges` returning (nil, nil) is correct Go).

## e) WHAT WE SHOULD IMPROVE

1. **Commit smaller, sooner in lint sweeps** — the E24 batch sits
   uncommitted across 13 files; the daemon window was real (sibling's
   scripts/ appeared mid-session). Explicit-path `git add` only.
2. **The `f` flag-set convention needs a recorded decision** — either
   policy-ignore in varnamelen config or rename to `flags` (§g1).
3. **reports/ durability gap is now load-bearing** — published release
   notes point into a gitignored path; needs either negation patterns
   outside the buildflow-managed block or a docs/ home for curated
   summaries.
4. **The findings-JSON-driven loop** (lint → group → batch → verify →
   refresh) was dramatically more efficient once adopted; make it the
   documented procedure for E24-class work.
5. **contextcheck in scanstore is a REAL finding** — `NewSink->record`
   drops ctx on the Observe path; fix belongs in the sibling's lane,
   coordinate before touching.

## f) NEXT (priority order, ≤50)

1. Commit + push the E24 batch (build green NOW; explicit paths).
2. varnamelen `f` policy decision (§g1) → encode or rename.
3. scanstore fixes (coordinate with sibling): contextcheck ctx
   pass-through, gocritic singleCaseSwitch, godoclint, cyclop
   LoadResumeSource (13→≤12), views omitempty, unparam failedResult.
4. `runReplay` cyclop 15: extract resolve-run + stream sections.
5. gocognit `runScan` 38: extract resume-skip, event-store-attach,
   render phases.
6. gocognit `domainFeeder.next` 30: extract per-streamer drain loop.
7. ~~gocognit `TestQueryReadModelsRoundTrip` 30: table/subtest split.~~ done at `399385a` (golangci-lint driven to zero)
8. ~~wsl_v5 ×7 (doctor_test ×3, render_test ×2, render.go, doctor.go).~~ done at `399385a` (golangci-lint driven to zero)
9. ~~embeddedstructfieldcheck cli_test:257.~~ done at `399385a` (golangci-lint driven to zero)
10. makezero scan.go:47 / cli_test:659 (verify FP vs real after format).
11. Final `buildflow format` + authoritative findings run → 0 remaining
    or documented nolint set.
12. Full `-race` suite ×1 post-batch (fast now, ~55s, soak opt-in).
13. AGENTS.md: record the findings-JSON lint procedure + LSP-staleness
    gotcha.
14. E25: add golangci-lint CI step (pinned action SHA).
15. E25: add govulncheck CI step.
16. E25: `-timeout 15m` belt in ci.yml Test step.
17. E25/owner: nightly GDD_SOAK=1 scheduled workflow (sibling §g1).
18. E25: consider GOTOOLCHAIN pin in ci.yml (sibling item 45).
19. Owner infra: rebuild buildflow binary for the 1.27.2 floor
    (BuildFlow#56) — unblocks pre-commit hook + buildflow-format lint
    steps (§g3).
20. ~~E22: verified-at headers on the 22 status reports.~~ won't implement — mechanized by the docs-gates freshness gate (extracted 2026-11-03) which flags stale pinned versions mechanically
21. E22: cqrs-htmx/go-cqrs-lite re-audit cycle.
22. E33: CLI coverage 69→90% (targeted cli.go tests).
23. E33: input-parser fuzz target (CSV quoting edges).
24. Homebrew tap: formula + LarsArtmann/homebrew-tap repo + README
    Install section update.
25. reports/ durability: decide negation-pattern vs docs/ move; fix the
    release-notes/CHANGELOG references if moved.
26. Census handoff: read sibling's next report; harvest their §f items
    into TODO_LIST (their scripts/unbound.sh landed — E7 lifecycle).
27. E7 NOTES.md belongs to the sibling; verify it lands, don't write it.
28. Nightly-soak decision record in ROADMAP once owner answers.
29. Consumer sweep for v0.1.0 (go-ecosystem-upgrade; likely none).
30. docs-gates full run post-batch (freshness after TODO strikes).
31. CHANGELOG [Unreleased]: decide whether lint/refactor work gets an
    Unreleased section (internal-only → probably no).
32. Retire `docs/status` index drift check (README.md index freshness).
33. Re-run `gh run list` at next session start (adopted habit — verify
    64c6b80's CI went green).
34. Verify E6 artifacts survive in reports/ after tmp reaper sweeps /tmp
    (they should — copied, not moved).
35. Consider `erraudit` pass (go-error-modernization skill workflow) now
    that AsType is in play — lint ./... --type-aware per module.
36. Sweep for remaining `fmt.Errorf` without %w that LOSE chains
    (doctor.go:47 style upgrades already done; look for %v-of-err).
37. E31 leftovers: ErrNoRuns store-path naming — DONE via replay.go
    wrapping this session; strike its TODO row after verification.
38. verify-before-filing: if any BuildFlow finding looks like an
    upstream bug, file it in BuildFlow repo (close-the-loop rule).
39. render_test.go golines/format leftovers — final format pass covers.
40. Check `git status` for daemon sweeps of the uncommitted batch RIGHT
    before committing (item 1).
41. pma daemon: verify it did NOT sweep the sibling's scripts/ into a
    junk commit meanwhile.
42. E26/E27/E30 regression check: formats enum + exit-code tests still
    pass after tag rewrites (they did in the 19:08 suite run).
43. README: mention `GDD_SOAK=1` for contributors running perf soaks.
44. STACK.md/AGENTS.md: record the sentinel-file convention
    (cli/errors.go pattern) for future commands.
45. After E24 zero: E25 lint step green check, then nightly soak PR.
46. pty/progress smoke after progress.go const changes (behavior
    unchanged, but the redraw interval is now a const — sanity run).
47. `go-cqrs-lite` version pin review (auto-upgrade daemon may bump).
48. Release-notes "Homebrew tap planned" — deliver or soften wording.
49. Cross-check `.golangci.yml` against a future auto-configure rewrite
    (project exceptions are policy; expect review on regeneration).
50. Session-close ritual: docs-gates + race + push + status-report
    annotation of THIS report by the next session.

## g) QUESTIONS (cannot answer myself)

1. **varnamelen `f` policy:** the CLI's `runX(ctx, f XxxFlags, …)`
   receiver-style `f` is a consistent convention across 6+ functions.
   Encode `f` (and possibly `d` for parsed domains) in varnamelen's
   ignore-names as recorded policy — or spend the churn renaming to
   `flags` on the cli lane (which the census sibling is actively
   touching)?
2. **E24 finish line:** gate at literal ZERO findings (deliberate sites
   get `//nolint:<linter> // reason` inline, ~8 of them) — or allow a
   small curated non-zero set via .golangci exclusions with rationale
   comments? (Zero-with-inline-nolint is my recommendation: the
   suppression lives at the site it justifies.)
3. **BuildFlow binary rebuild:** the go1.27.0-built buildflow binary
   fails govalid/govulncheck/golangci-inside-format against the repo's
   1.27.2 floor (BuildFlow#56). Is rebuilding/refreshing that binary
   mine to do now (it unblocks the pre-commit hook AND E25's lint
   story), or parked with you as owner tooling?

---

_Point-in-time snapshot 19:08 CET, tree uncommitted-but-green (E24
batch), `64c6b80` pushed to both remotes. Census sibling ACTIVE
(scripts/unbound._ at 14:54). E7/E6 artifacts archived in reports/.*
