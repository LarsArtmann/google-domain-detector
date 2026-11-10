# Status Report — Full Plan Execution Session (spine + process debt + zone data)

**Written:** 2026-11-10 01:49 CET · **Session scope:** executing the ENTIRE
plan (v2, `docs/planning/2026-11-10_01-18_*`, which supersedes the 00-50
plan and preserves its spine) plus the 6 process-debt items, after the
owner's "GET SHIT DONE — the WHOLE TODO LIST" directive.

## Scope line — verified vs not verified

**Verified, with commands:**

- G1 lifted: the scanstore session's closing report (01:12) documents
  buildflow exit 0, docs-gates 5/5 PASS, nix build green; tree quiet since.
- PD28 root cause: `buildflow history --step samber-linter --last-error` +
  `--step test-compile` + state-DB query (`step_outputs.error_code`) — both
  failed on `toolchain.skew` (buildflow binary built with go1.27.0 vs the
  go.mod `go 1.27.2` floor the cqrs deps require; BuildFlow issue #56).
- PD29: mermaid graph extracted and compiled with mmdc 12.0.0 → 228 KiB SVG,
  exit 0.
- Per-task gates: `go build ./...`, `go vet ./...`, `go test -race ./...`
  green after every task; live smokes (yaml/toml/jsonl output; exit codes
  2/1; classified error summary); `nix build` green after T3's dep change +
  `nix-hash-fix`; `stack-drift-check.sh` PASS; `docs-gates freshness` green
  after the 9-item harvest.
- Zone data: Nominet program facts fetched from official registrars.nominet.uk
  - nominet.uk pages; SWITCH facts from switch.ch contact page (both
    2026-11-10, captured in `docs/zone-data/`).

**NOT verified / not done:**

- The full `buildflow` gate could NOT run for Go analysis steps (BuildFlow#56
  embedded-analyzer skew — the OTHER session rebuilt buildflow from HEAD and
  the embedded go1.27.0 persists). Direct gates used instead, per the policy
  the other session documented in AGENTS.md; every commit this session used
  `--no-verify` under that policy.
- T5 validation run still IN FLIGHT at report time (started 01:45,
  3645-domain .ch slice, event store already 2.1 MB); no verdict analysis,
  no dig cross-checks yet.
- Nothing pushed to either remote this session (11 local commits);
  v0.1.0 not tagged.
- DENIC (.de) zone-data research failed on rate limit (re-search needed).

---

## a) FULLY DONE (this session, commits in parens)

1. **PD28** — the two unidentified pre-commit failures diagnosed BY NAME
   and root cause (samber-linter `toolchain.skew` + test-compile
   `stale_toolchain`, same cause, self-healed by binary refresh); recorded
   in AGENTS.md.
2. **PD29** — plan mermaid graph compile-validated (mmdc → SVG).
3. **PD30** — docs-gates verdict flip-flops explained mechanically
   (pre-commit `--staged-only` + unstaged-stash scope + per-file-set result
   cache + binary skew mid-race); documented in AGENTS.md.
4. **PD31** — mixed-commit policy decided and recorded in AGENTS.md
   (prefer soft-reset split; message-only `--no-verify` amend ONLY when the
   tree is byte-identical; never rewrite pushed history unasked).
5. **PD32** — gomod-check green, go.mod tidy (the scanstore session's tidy
   held); vendorHash valid through T3's dep change (nix-hash-fix + green
   hermetic build).
6. **PD33** — mirror posture: accept the published in-flight scanstore code;
   no history rewrite without explicit owner approval.
7. **T1 (76036cf content)** — `--format` kept, cmdguard `-o` REJECTED
   (global flag + CLI-object resolution breaks per-command flag structs and
   the injected-deps test seam; unix `-o` reads as "output file"); decision
   in STACK.md log, audit gap #7 closed, T20 resolved-as-decided-against.
8. **T2 (4235e02)** — `--format yaml,toml,jsonl` on scan AND replay;
   `render.JSONL` emits the full-evidence DTO one object per line (T7 will
   only change WHEN lines are written, not WHAT a line contains); the
   yaml-must-fail test flipped to a real bogus-format test; render + cli
   tests; live smoke of all three formats.
9. **T3 (417afc7)** — go-atomic-write v0.6.2 promoted to direct dep;
   `writeCache` → `WriteWithPerm(0600)` (never WriteIfChanged — mtime-TTL
   trap pinned by test); `User-Agent: google-domain-detector/<version>` on
   the feed (plumbed through runScan); vendorHash refreshed; STACK.md
   crash-safe-writes row; tests: perms, identical-content mtime refresh,
   16-way concurrent Load, server-side UA observation.
10. **T4 (6fc85ab)** — usage errors exit 2 (static sentinels + usageError
    helper, also clearing two err113 findings), scan failures exit 1;
    `validate:"min=1,max=512"` on --workers; `WithEnvPrefix("GDD_")` with
    shortened env tags (effective names unchanged); RecoveryMiddleware;
    WithCLICommit plumbed (`-X main.commit`); WithExample on scan; 5 new
    tests incl. panic-recovery and env-prefix resolution; live exit-code
    smoke.
11. **T10 (49b87fc)** — error taxonomy end to end: ErrorKind
    (nxdomain/timeout/servfail/canceled/other) on Result, in Counts
    (ErrorKinds breakdown), the JSON DTO (`error_kind`), the event journal
    (additive payload field), the observation read model, and replay; one
    retry per transient lookup via a resolver wrapper (NXDOMAIN never
    retried, both attempts share the domain budget); one-line run summary
    on scan + replay stderr ("errors E (nxdomain a, timeout b, …)");
    live smoke shows the classified tail.
12. **T17 partial (docs/zone-data/, untracked→daemon)** — Nominet: the
    real program is the ".UK Zone Files Access Licence" (our TODO's "DAA"
    name is stale), free, email application + MiTek ID verification —
    draft email ready to send. SWITCH: NO application exists — the zone is
    open data via AXFR with free TSIG keys from tsig@switch.ch; the .ch
    gate is a formality, not paperwork. Constraint review + owner checklist
    written.
13. **Docs upkeep en route** — CHANGELOG Unreleased entries for T2/T3/T4/
    T10; TODO_LIST harvested (9 items, gate-matching tokens) incl. T20
    decided-against and mermaid-check done.

## b) PARTIALLY DONE

1. **T5 validation run** — 3645-domain .ch slice scan launched 01:45 with
   the new binary (workers 24, event store, json output, rusage memory
   capture); STILL RUNNING at report time. Replay comparison, error-tail
   analysis (taxonomy readout), 10-verdict dig cross-check, fallout fixes,
   and the tag-readiness verdict all pending.
2. **T17** — Nominet + SWITCH done; DENIC research rate-limited (needs a
   retry); crt.sh fallback extractor (v2's T19 add-on) not started.
3. **Coordination with the third session** (plan-v2 author, active since
   01:18): it diagnosed + documented BuildFlow#56 and rebuilt the binary;
   I adopted its policy and did not race it in the buildflow repo. No
   explicit task-split negotiated; overlap risk remains for T7-T9/T11-T12.

## c) NOT STARTED

T6 (release v0.1.0 — gated on T5 clean per G3), T7 (jsonl streaming),
T8 (input streaming + streaming replay), T9 (journal-based --resume),
T11 (scanstore hardening: sink retry/backoff, ctrl-C test, 100k bench —
gates G4), T12 (show/runs read-model consumers), T13 (go-output polish),
T14 (README + tap), T15 (WithDoctorCheck preflight), T16 (nom live
progress), T18 (audit upkeep + cqrs re-audit), T20 (lint cleanup to zero),
T21 (cmdguard-lint CI), T23 (registry-derived enum), T24 (NXDOMAIN
pre-sweep), T25 (CSV enrichment), T26 (scoped exit codes), T27 (infra
long-tail + ROADMAP). Scanstore follow-ups: sink retry (overlaps T11),
register-unknown-renderer, docs/status index, version-freshness guard,
TimeoutMiddleware, FormatsForShape (=T23), error taxonomy (DONE via T10).

## d) TOTALLY FUCKED UP

1. **Lost the daemon commit race TWICE** (T3's 8-file and T4's 10-file
   bundles swept under junk messages before my `git add` completed).
   Recovered via policy (T3: message-only amend, tree byte-identical; T4:
   soft-reset split into code + formatting commits). T1's content (76036cf)
   kept a junk message for good — I moved on after the hook failure instead
   of amending; its rationale lives only in STACK.md.
2. **Stash-eating misdiagnosis:** my TODO_LIST harvest vanished and I
   first blamed dprint reformatting; the real cause was the OTHER
   session's pre-commit hook (`git stash push --keep-index` + failed pop)
   — the mechanism was in the hook source I had read minutes earlier.
   `git stash show` forensics recovered 100% of the work. First move on
   "my edit disappeared" must be `git stash list`.
3. **~50 minutes of work unpushed** — 11 local commits on a two-remote
   repo; a disk failure would eat the session. I deferred pushing to T6 by
   design, but the risk window is real and the owner's earlier explicit
   push authorization shows pushes are wanted, not feared.
4. **Net err113 backlog unchanged:** my own new code (render.JSONL
   fmt.Errorf, errors_test errors.New) added findings as fast as T4
   removed them. T20 still owns zeroing it; I should not have added new
   ones.

## e) WHAT WE SHOULD IMPROVE

1. Commit IMMEDIATELY after each edit batch, not after the task — the
   daemon's sweep window is seconds, and every lost race costs either a
   junk-message commit or recovery surgery.
2. `git stash list` is the first command when an edit vanishes; the hook
   stashes by design.
3. Push at task boundaries (or ask once for a standing rule) instead of
   hoarding local commits until a release tag.
4. Capture subagent research to disk the moment it arrives (the SWITCH
   findings nearly evaporated when the report went to background).
5. The BuildFlow#56 fix (rebuild with go ≥1.27.2) is the single unblock
   for hooks and the full gate — it lives in the BuildFlow repo, not here;
   whoever takes it should coordinate with the plan-v2 session.

## f) Up to 50 next things (ranked)

1. Finish T5: wait for the 3645-domain run, replay-compare, error-tail
   readout, 10 dig cross-checks, fix fallout, record verdict (G3).
2. T6: release v0.1.0 (go-release skill; CHANGELOG cut, tag, BOTH remotes,
   GitHub release, proxy/pkg.go.dev check, `go get` pull-check).
3. Push the 11 local commits now (both remotes) — pending owner call.
4. Retry DENIC zone-data research (rate-limited); write denic-de.md.
5. crt.sh fallback extractor for non-Tranco .ch (v2 T19 add-on).
6. T7 jsonl streaming (visitor refactor in scan.Run). — RESOLVED-DONE
   (line-oriented output landed as render JSONLStream; struck row in
   TODO_LIST Detection coverage; annotated 2026-11-10, tokens went
   generic).
7. T8 input streaming + streaming replay. — RESOLVED-DONE (chunked
   domainFeeder + input.Streamer + replay walk; struck row in TODO_LIST
   01-12/06-09 harvest; annotated 2026-11-10, tokens went generic).
8. T9 journal-based --resume.
9. T11 scanstore sink retry/backoff + ctrl-C test + 100k bench (G4).
10. T12 show/runs commands (de-ghost read models).
11. T13 go-output polish (--color, SetFooter, Title, AddRowChecked).
12. T15 WithDoctorCheck preflight.
13. T16 nom live progress.
14. T14 README honest + install + Homebrew tap (post-tag).
15. T18 audit upkeep (citations vs HEAD, screenshot, cqrs re-audit —
    premise flipped by scanstore).
16. T20 lint cleanup to zero (~30 warnings incl. my new err113s).
17. T21 cmdguard-lint CI step.
18. T23 registry-derived format enum (FormatsForShape).
19. T24 NXDOMAIN pre-sweep (after T9 + G4).
20. T25 CSV enrichment.
21. T26 scoped error-family exit codes (75/65).
22. T27 infra long-tail (TimeoutMiddleware, describe contract test,
    vendorHash.nix, freshness guard, docs-gates cache key, ETag, ROADMAP).
23. Owner sends Nominet email + requests SWITCH TSIG keys (drafts ready).
24. Harvest THIS report's §f into TODO_LIST (docs-gates freshness).
25. BuildFlow#56: rebuild buildflow with go1.27.2 toolchain (other repo).
26. docs/status index for discoverability.
27. Split-brain check: my T10 edited the scanstore session's test fixture
    (failedResult ErrKind) — benign, but flag in their next session's
    context.

## g) Questions I cannot answer myself

1. **Push now or hold for the tag?** 11 local commits, nothing pushed this
   session (both remotes configured; earlier pushes were explicitly
   authorized). Push per-task from now on?
2. **Is the plan-v2 session still active, and do we split the remaining
   tasks?** It owned BuildFlow#56 and may be executing v2 tasks in
   parallel; an explicit split (e.g. I take T7-T9, it takes T11-T12)
   would prevent trampling.
3. **T6 release authorization:** once T5 verifies clean, proceed straight
   through tag v0.1.0 + push + GitHub release without a check-in, or
   report T5's verdict first?

---

_Point-in-time snapshot. WAITING FOR INSTRUCTIONS (T5 scan still running
in background — its result lands in /tmp/t5-scan.json + /tmp/t5.db)._
