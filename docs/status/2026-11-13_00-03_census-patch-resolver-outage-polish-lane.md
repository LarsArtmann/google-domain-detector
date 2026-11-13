# Status Report — Census Patch Wave, Resolver Outage, and the Polish Lane

**Written:** 2026-11-13 00:03 CET · **Session covered:** 2026-11-12
23:00 → 2026-11-13 00:03 (the "GET SHIT DONE, THE WHOLE TODO LIST"
execution session: code batch 1+2, census patch launch, lint-to-zero).

## Scope line — verified vs not verified

**Verified with commands:** `go build ./...`, `go vet ./...` clean;
`go test ./... -count=1` all 9 packages ok (incl. `-race` full-suite green
at batch-1 and batch-2 boundaries); `golangci-lint run` → **0 issues**;
`nix build` exit 0 (at batch-2 boundary); census-gate regressions against
the real wave-A artifacts (gap → exit 4, reconciled → exit 0) plus three
synthetic forms (interrupted diagnosis, resume diagnosis, zero-error
summary). Census patch wave executed detached and COMPLETED (pass 1 +
3 cleanup rounds + journal snapshot) — verified via monitor.log and the
snapshot tarball.

**Not verified:** the three unpushed commits' CI run (not pushed yet —
ahead 3 at report time); waves B/C not launched; the lint-zero state was
verified only with the local golangci-lint binary (buildflow's own lint
gate remains blocked by BuildFlow#56).

---

## a) FULLY DONE (this session; all committed, most pushed)

1. **Session-start ritual + corrected todo list.** Baseline green (build,
   vet, tests, CI status via `gh run list` — all green, nix build green).
   The stale 2026-11-10 todo list was replaced with the live 31-item list
   (T1–T12 marked complete, census + polish lanes opened).
2. **go-ndjson evaluation (owner question, answered with evidence).**
   NOT worth adopting: this repo only *writes* ndjson
   (`render.JSONLStream`/`encodeJSONLLine`); nothing anywhere reads it
   back (`rg` over all Go sources: zero read paths; consumers are jq/CRM
   scripts). go-ndjson is a reader with format auto-detection — a
   dependency with no call site. Revisit only if a `merge`/re-ingest
   command ever lands.
3. **Interrupted-run summary double-count fixed (Operational row).**
   `classify.Result.Unscanned` marks placeholders for input domains a
   canceled run never reached; `Counts.Tally` skips them entirely; the
   stderr summary now counts scanned-only and prints
   `interrupted: N input domains were never scanned (canceled before
   their turn)`. Consequence: a SIGINT'd wave's "N domains:" line no
   longer fakes complete coverage — the gate's manifest reconciliation
   now catches interrupted runs as exit-4 gaps. Pinned by
   `TestScanInterruptedSummaryCountsScannedOnly` (stability-tested 8/8
   after fixing a post-cancel queue race with a grace-hold in hangRes),
   `TestRunMarksOnlyNeverReachedDomainsUnscanned`,
   `TestTallySkipsUnscannedPlaceholders`. The journal's scan.completed
   tally now also excludes placeholders.
4. **Resume-skip accounting at the source (Detection-coverage row).**
   `domainFeeder` counts skip-listed domains; the run prints
   `resume: skipped N input domains already observed in run X` on stderr
   (`reportInputAccounting`, extracted during lint-zero). Pinned inside
   `TestScanResumeSkipsObservedDomains`. The wave-A class of silent
   2 731-domain hole can never be silent again.
5. **census-gate.sh hardened + www-normalized (three fixes, all
   regression-verified).**
   - Parses zero-error summaries (`errors 0` prints no kind suffix;
     before: sed passed the line through unmatched → `set -u` crash on
     clean runs — latent bug found via a synthetic test).
   - Coverage-gap diagnosis now surfaces the scan's own accounting lines
     (interrupted count, resume-skip count) and reports `delta N`.
   - `--manifest` reconciliation normalizes manifest lines like
     `domain.Parse` does (lowercase, trailing dot, one leading `www.`)
     before uniquing: `www.foo.ch` + `foo.ch` = ONE target, matching the
     feeder's dedup — no false coverage gap. Measured first: all three
     frozen manifests have ZERO normalization collisions today (delta 0),
     so this protects future zone-pull inputs without changing current
     accounting.
6. **www-strip policy decided + documented.** Normalize-and-scan, even
     when the stripped name IS a public suffix (`www.edu.cn` → `edu.cn`,
     26 cases in wave A scanned fine; a hosted suffix apex has real DNS
     evidence). Recorded as a `### www-strip policy` section in
     docs/census.md with the gate consequence.
7. **IPv6 resolver handling proven (E31 row).** `resolverDialAddr`
     extracted (bare v6 literals bracketed, bracketed host:port passes
     through) + 7-case table test + a REAL end-to-end v6 test: one-shot
     DNS responder on `[::1]:<port>`, real query through
     `NewWithAddr`, asserts the parsed empty-response DNSError. Two
     discoveries on the way: Go maps a NOERROR/0-answer TXT response to
     `IsNotFound` (not nil,nil); this host runs a live resolver on
     `[::1]:53` (my first test accidentally queried it and got REAL
     example.com TXT records — proof the v6 dial path works).
8. **TXT truncation investigation CLOSED with live evidence.**
     cloudflare.com's 2 539-byte TXT response (the "cannot unmarshal"
     class suspect) parses cleanly through the healthy forwarder — full
     SPF + site-verification evidence, zero errors. The historical
     errors were resolver-distress artifacts (same family as the
     lying-NXDOMAIN finding), not a truncation bug. Row closes.
9. **scan --max-runtime whole-run budget (E31 core).** Deadline on the
     entire run (feeding, scanning, recording); expiry classifies
     in-flight as timeout, unstarted as interrupted — composes with the
     accounting fix. Pinned by `TestScanMaxRuntimeBudgetExpiry`
     (ctx-aware slow resolver, stability-tested).
10. **T16 delta query VERIFIED LIVE (row was already implemented by a
     prior session — `replay --since` + FlipDiff).** Ran it on the real
     census journal: wave-A final vs the 2026-11-10 poisoned full run =
     **4 flips** (withgoogle.com, bsky.app, opendns.com… — resolver-
     distress false negatives correctly detected in the healthy run),
     8.7 s wall, correct stderr tallies. Row closes as
     done-by-prior-session + verified-now.
11. **enrich command (census T18 / E29, journal-backed).** Streams a
     lead CSV through the read model, preserves every original column
     and row, appends `gdd_domain,workspace,gcp,error`. Unparseable
     cells and never-scanned domains keep their rows with explanatory
     error values (enrichment never drops rows); the header join reuses
     a newly exported `input.HeaderColumn` (one owner for the
     domain-column convention). Two tests (join semantics + usage
     contract exit 2).
12. **Census binary gdd-20261112 built + provenance.** Clean-tree build:
     `vcs.revision=308c32d` (= HEAD at build), `vcs.modified=false` —
     first fully-clean census build; sha256 + behavior deltas recorded
     in reports/census/unbound/PROVENANCE.md (detection rules UNCHANGED,
     Ruleset v1-2026-11-10 — no ruleset bump required or taken).
13. **Wave-A patch wave EXECUTED end-to-end.** Manifest derived from
     soak3.jsonl (`jq .domain | sort -u`): **exactly 2 731 domains** —
     matches the gate's gap to the domain. Frozen as
     `tranco1m-a-patch.txt` + sha256 into MANIFEST.sha256. Launched
     detached via census-wave.sh (fresh pass, no --resume, w4/10s,
     forwarder). Pass 1 + 3 cleanup rounds ran; journal snapshotted
     (`tranco1m-a-patch-complete-20261112-2353.tar.gz`, 349 MB db,
     56 MB tar + sha256 sidecar).
14. **Lint-to-zero (E24).** 28 findings → **0** (`golangci-lint run`
     exit clean). Flag-struct tags rewritten per the in-repo policy
     (alphabetical, unpadded, validate-first) with the documented
     golines nolints restored to their original standalone-comment form;
     cyclop resolved by extracting `applyRunBudget`/
     `reportInputAccounting` (runScan) and `openEnrichInput`/
     `enrichRows` (runEnrich); `enrichColumns` global → func; magic CSV
     line offset → `csvFirstDataLine` const; test findings fixed with
     assertive forms.

## b) PARTIALLY DONE

1. **Wave-A patch residual: 25/2 731 unknown (0.92%) vs the 0.5 % bar.**
   Pass 1 and cleanup round 1 ran DURING a host-level outbound UDP:53
   outage (see d1): pass 1 = 2226 SERVFAIL of 2 731; round 1 = 1222
   SERVFAIL. Rounds 2–3 ran as UDP recovered; residual fell to 25
   (0.92 %, bar 13.7 domains). The runner exited 1 (over bar) and
   snapshotted. One more cleanup round through the now-healthy
   forwarder, or a `--servfail-stable` assessment, closes it — the
   runbook's own ≥2-rounds-through-verified-healthy rule says do the
   round first.
2. **Pushes: ahead 3** (lint-zero commit + 2 daemon sweeps). Not pushed
   at report time; CI on them not yet observed.
3. **docs-gates: `audit-all` and `cadence` red** (pre-existing; checked
   at the batch-2 boundary — belongs to the pending audit-upkeep/docs
   work, deliberately not chased mid-lane).

## c) NOT STARTED (this session; from the corrected 31-item list)

Waves B (400k) and C (500k); the patch's post-outage cleanup round;
census report (T15); unknown-cleanup loops for B/C; .ch SWITCH TSIG
zone pull + corpus build; crt.sh .de rate-limit-aware scheduling;
Nominet DAA draft re-surface/staging; cmdguard-lint CI step;
golangci-lint + govulncheck CI steps (E25); version-freshness guard for
pinned audit reports; audit upkeep (re-verify citations vs HEAD,
re-audit go-cqrs-lite verdicts); `go mod tidy` sweep; input-parser
fuzzing (E33); cli coverage 69 % → 90 % (E33); own-resolver guidance
doc + buffered-format bulk note; retract-note dry run; README polish
(asciinema/GIF + feed-freshness note, E20); TODO_LIST rewrite + row
closures (CAA→ROADMAP pointer, FormatsForShape duplicate, consumer
propagation sweep, nxdomain-bucket re-verify, TXT truncation — all
decided/closable); AGENTS.md updates for this session's findings; final
push of both remotes.

## d) TOTALLY FUCKED UP

1. **Launched the patch wave without a real upstream pre-flight — and
   the repo's own tool would have caught it.** `unbound.sh status`
   printed "health: answering queries" while the upstream was DEAD
   (host-level outbound UDP:53 outage, 23:24–~23:45: dig @9.9.9.9
   direct = "no servers could be reached", TCP:53 OK, ICMP OK; recovered
   by 00:03). The status check proves liveness, not upstream health —
   unbound answered the health probe from cache. `gdd doctor` (real 5 s
   lookup through the target resolver) was sitting right there, wired,
   tested, GDD_RESOLVER-aware — and I didn't run it. Cost: pass 1 and
   cleanup round 1 (≈3 450 garbage observations) burned against a dead
   upstream; the journal carries them as unknowns (excluded from census
   accounting by design, but two of three cleanup rounds were wasted
   during the outage window).
2. **The outage went unnoticed for ~30 minutes of wall clock** because I
   was heads-down in the lint lane after launching detached. The 30-min
   progress poller logged pass 1's storm only at 23:54 (after cleanup
   had already finished); I first read the FAIL at 23:36. For a 5-minute
   wave the poller cannot protect anything — only a pre-flight can.
3. **Mechanical stumbles (all caught by tests or regression runs, none
   shipped):** a no-op multiedit (old_string == new_string) on
   census-gate.sh; an edit that glued `exit 4` onto an echo line in
   coverage_check (caught by the wave-A regression: gate printed FAIL
   AND PASS, exit 0); an edit that swallowed a doc-comment line in
   cli_test.go (repaired); the enrich test seeded `workspace.example`
   against a fixture that serves `acme.com` (the assertion caught it);
   ~6 edit-tool mtime rejections against the auto-commit daemon
   (re-read + re-apply each time).
4. **Chased tagalign/golines in circles** before reading the in-repo
   policy: the `.golangci.yml` comment documents the exact conflict and
   the nolint form; git history had the working placement. Three
   iterations (inline nolint → nolintlint "unused" → history dig →
   standalone comment form). Rule violated: read the config and the
   history BEFORE fighting a linter.

## e) WHAT WE SHOULD IMPROVE

1. **Pre-flight gate before EVERY wave launch** (law candidate for
   census.md + census-wave.sh): (a) `gdd doctor --resolver
   127.0.0.1:5353`-equivalent — a real uncached lookup through the
   target resolver, (b) a 2–3-domain dig spot-check including one
   guaranteed-uncached name. `unbound.sh status` is liveness, not
   health. The E7/E9 ladder paid for this lesson once; tonight it was
   re-bought for free because the check wasn't wired into the runner.
2. **Read config policy + git history before fighting linters** — the
   answer was in-repo both times (tagalign order, golines nolint form).
3. **For short waves (< poller interval), the only protection is the
   pre-flight**; consider adding a pass-1 early-abort: if the first 100
   scanned domains exceed ~50 % unknown, abort the pass (data is
   cheaper than rounds).
4. **Document the outage-fallback path:** unbound-tcp.conf (TCP-upstream
   forwarder, port 5354) already exists from a prior session's
   experiment; a live invocation recipe in census.md ("if UDP:53 dies:
   start TCP-mode unbound, point GDD_RESOLVER at 5354") would have
   turned tonight's scramble into a runbook step.
5. **Daemon mtime discipline:** on any file not touched in the
   immediately-preceding batch, View immediately before Edit; count on
   ~1 extra round trip per file.

## f) Next things (ranked; 27 rows — the corrected list minus what a/b closed)

1. One more cleanup round on tranco1m-a-patch through the now-healthy
   forwarder (residual 25 → under 13.7); only then consider
   `--servfail-stable` for whatever remains.
2. Push the 3 unpushed commits to both remotes; watch the CI run
   (release law: green on the pushed commit).
3. Wire the pre-flight into census-wave.sh (doctor-style real lookup +
   uncached dig; refuse to start on failure) — before wave B.
4. Launch wave B (400k, ~3 days at 1.5 dom/s) via census-wave.sh
   detached; decide chain-vs-checkpoint (question g2).
5. Wave C (500k, ~4 days) after B gates PASS.
6. Unknown-cleanup loops per wave (< 0.5 % bar, falling workers).
7. Census report (T15): journal-derived, archived under reports/,
   wave ledger updated with patch + B + C.
8. Kill the 2026-11-10 zombie shell (pid 1229551, unbound-tcp wrapper).
9. Outage fallback recipe in census.md (TCP-mode unbound on 5354).
10. AGENTS.md gotchas: UDP:53 outage incident + doctor-pre-flight law +
    the ::1:53 system resolver discovery.
11. .ch zone pull via SWITCH public TSIG (docs/zone-data/switch-ch.md;
    once/24h) + corpus build → feeds E28 pre-filter.
12. NXDOMAIN/A-existence pre-sweep before whole-TLD scale (E28, gated).
13. crt.sh .de extractor rate-limit-aware scheduling (once/day per TLD).
14. Nominet DAA draft: re-surface + stage for the owner (g3).
15. cmdguard-lint as a CI step (expect zero findings).
16. golangci-lint + govulncheck CI steps (E25).
17. Version-freshness guard: flag audit reports whose pinned lib
    released a newer tag (extend scripts/audit-freshness.sh).
18. Audit upkeep: re-verify docs/review/* citations vs current HEAD;
    re-audit go-cqrs-lite/cqrs-htmx verdicts post-scanstore.
19. `go mod tidy` sweep (post-feature-stabilization — now is that
    time).
20. Input-parser fuzzing (E33; CSV quoting/encoding edges — a fuzz
    corpus stub already exists in internal/input).
21. cli package coverage 69 % → 90 % (E33; per the coverage-gap list).
22. Own-resolver guidance doc for whole-zone scale (unbound/knot; the
    "public resolvers throttle" row) + buffered-format bulk note
    (jsonl-only at scale) — README/docs lane.
23. Retract-note dry run (go-release lane; documentation-only
    emergency procedure rehearsal).
24. README polish: asciinema/GIF + feed-freshness/UA note (E20).
25. TODO_LIST rewrite: close T16 (verified), T18 (enrich), www-strip,
    resume-accounting, summary double-count, TXT truncation, IPv6,
    max-runtime, lint-zero; mark patch status; refresh all sections.
26. Close decided rows: CAA (→ ROADMAP pointer, no speculative probes),
    FormatsForShape (superseded by SupportedFormats + E27 test),
    consumer-propagation sweep (no external consumers yet), nxdomain
    re-verify row (superseded by T09 recovery + unbound runs).
27. Final: status report + push both remotes + AGENTS.md refresh.

## g) Questions only the owner can answer

1. **The UDP:53 outage:** do you know what killed outbound UDP :53
   from this host between ~23:24 and ~23:45 CET (VPN toggle? router/
   ISP? something you changed)? It recovered on its own by 00:03;
   TCP:53 and ICMP never blinked. This decides whether waves B/C need
   the TCP-upstream unbound as a standing fallback or a retry-on-fail
   pre-flight is enough.
2. **Wave chaining:** launch B and C as one detached chain (C starts
   only if B's runner exits 0 = gate PASS, unknown-cleanup included),
   or checkpoint between waves for your review before C? Chain = zero
   idle machine time over ~7 days; checkpoint = a human reads B's
   health before 500k more domains burn.
3. **Nominet .uk DAA draft** (ready since 11-10, owner-send item):
   re-surface and stage for sending now, or park until the census
   completes? (SWITCH .ch needs nothing from you — I can pull the zone
   with the public TSIG keys whenever the corpus work starts.)
