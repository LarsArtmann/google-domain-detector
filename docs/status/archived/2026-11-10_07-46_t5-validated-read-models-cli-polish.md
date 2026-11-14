> **Live remainder:** every item resolved as of the 2026-11-13 docs-health pass (docs-gates green); open census work lives in TODO_LIST.md + docs/census.md.

# Status Report — google-domain-detector: T5 validated, read models consumed, CLI polish

> **CORRECTION (2026-11-10 10:40, E1 replication):** this report's
> "482/538 (89.6%) genuinely dead" nxdomain conclusion is RETRACTED. The
> re-verification re-scanned the bucket through Quad9 minutes after the
> poisoning window — it reproduced the same stale negative-cache answers
> and measured cache state, not domain liveness. The 10:10 E1 replication
> run sampled 11/12 "nxdomain" results ALIVE (own SOA + NS; authoritative,
> Quad9, and the local resolver all answer positively minutes later).
> Positive verdicts stand (14/14 dig-confirmed). See AGENTS.md gotcha
> "A distressed local resolver can LIE NXDOMAIN" and the E1 artifacts
> (reports/2026-11-10-e1-validation/, local copy).

**2026-11-10 07:46 CET** — session scope: continuation of the "WHOLE TODO
LIST" execution mandate (plan v2). Started from the 01:49 WAIT state;
reconstructed after /tmp wipe; discovered and absorbed the concurrent v2
session's landed work. NOT a whole-project audit.

**Scope line — what this report verified, and how:**

- Verified by fresh command this session: `go build ./...`, `go vet`,
  `go test -race ./...` (full suite, before T13), `go test` render+cli
  after T13; live 3645-domain scan + journal replay + set/diff compare
  (python); 12 dig cross-checks via Quad9; 925-domain error-tail re-sweep;
  538-domain nxdomain re-verification via `--resolver 9.9.9.9`; live
  smoke of `--color always` + footer + show/runs against the real binary.
- Verified by reading (not by command): the concurrent v2 session's
  claims-to-code mapping (T7/T8/T9/T11 test names and implementations
  exist and pass under -race in my runs).
- NOT verified: GitHub Actions (never run), the 06:09 toolship session's
  mass-scan numbers (taken from their report + archived artifacts),
  anything in the BuildFlow repo (their fix, their blocker).

## a) FULLY DONE (this session)

1. **Session recovery after /tmp wipe**: all T5 artifacts were gone
   (/tmp cleared). Rebuilt input from the durable archive
   (`reports/2026-11-09-tranco-de-ch-uk/targets.txt` → 3645 .ch domains,
   byte-count matches the prior session's list) and moved every new
   artifact to `reports/2026-11-10-t5-validation/` — the "durable from
   minute one" lesson applied.
2. **Concurrent-work mapping**: git-log + code read established that the
   v2 session ALREADY landed T7 (render.JSONLStream + completion-order
   streaming + cancel-mid-stream test), T8 (chunked domainFeeder +
   StreamObservations streaming replay), T9 (`--resume`, commit 3674285),
   T11 (sink bounded retry/backoff + loud dropped-count + ctrl-C drain +
   100k-event journal test + 10k stream test). My session did NOT redo
   them; verified green under `-race` in my full-suite run.
3. **T5 validation run (GATE G3) — executed and PASSED**:
   - Scan: 3645 .ch domains, 24 workers, system resolver, event store on.
     Result: workspace certain 325 / likely 140, GCP 55, errors 1463
     (nxdomain 538, timeout 819, servfail 106) — the error tail is
     EXPLAINED by T10's taxonomy, not a mystery bucket.
   - Journal round-trip: replay rebuilt 3645/3645 domains, zero set
     difference, zero content difference (byte-identical DTOs). The
     journal is trustworthy at this scale.
   - dig cross-checks (Quad9, independent resolver): 5/5 workspace-certain
     confirmed (Google MX), 2/2 workspace-likely confirmed (SPF include,
     non-Google MX), 2/2 GCP confirmed (34.x GCP IPs).
   - NXDOMAIN quality: initial 2-of-3 sample looked like false NXDOMAINs
     (dotcon.ch, dns1.ch resolve at Quad9 NOW); full re-verification of
     all 538 via `--resolver 9.9.9.9` at 4 workers: **482/538 (89.6%)
     genuinely dead**, 2 were GCP-live all along, 54 timed out. So the
     nxdomain bucket is ~90% true; false NXDOMAINs under resolver stress
     are REAL but rare (~1-2% of the bucket in this run).
   - Error-tail recovery sweep (925 timeout/servfail domains at 6
     workers): 78% recovered; found **+9 certain, +3 likely, +5 GCP**
     hidden in the tail; residue 191 errors (5.2% of the TLD slice).
   - Memory: MAXRSS 70 MB on the 925-domain pass — flat, streaming works.
   - **G3 verdict: PASS.** Detection verdicts are accurate and
     independently confirmed; the journal replays byte-identically; every
     error is decomposed; the tail is recoverable with the tool itself.
     Caveat recorded for docs: 24 workers against the local resolver
     saturates it (819 timeouts); bulk runs should use fewer workers or a
     clean `--resolver`.
4. **T17 zone-data (research half)**:
   - `docs/zone-data/switch-ch.md` REWRITTEN from verified facts
     (fetched zonedata.switch.ch directly): public TSIG keys on the page,
     exact `dig -k ch_zonedata.key @zonedata.switch.ch ... AXFR ch.`
     command, once-per-24h limit, Ordinance-on-Internet-Domains purpose
     clause, Switch CERT contact. The old "email tsig@switch.ch" path was
     stale; .ch needs NO owner action.
   - `docs/zone-data/denic-de.md` NEW: definitive research — **DENIC
     refuses ALL third-party zone access** (FAQ verbatim; no program, no
     fee, no contact exists). The plan's "DENIC RDS application" premise
     was wrong; .de full coverage is impossible, fallback = Tranco +
     crt.sh. Also found bonus SWITCH datasets (monthly top-1000 CSVs).
   - `docs/zone-data/README.md` checklist updated (.ch actionable now).
5. **T12 read-model consumers — show/runs (commit 80c1c60)**:
   - `scanstore/queries.go`: LatestObservation (TypedReader point lookup)
     - ListRuns (scan, newest-first).
   - `show <domain>`: latest observation, verdicts derived from evidence,
     observation count, last run; any output format; usage errors exit 2.
   - `runs`: run list with status, wave/resumed-from provenance, tallies.
   - Additive read-model evolution: RunView +StartedAt/Wave/ResumedFrom;
     fixed ObservationView fold dropping ErrKind (latent bug — the view
     always had the field, the fold never set it).
   - Tests: 6 CLI tests + SQLite round-trip query test (two runs, reopen,
     await-based projection catch-up).
6. **Data race found in the wild and killed**: the full `-race` suite
   exposed `scanChunkSize` as a package-global that a chunk-boundary test
   mutated while parallel tests read it (race report pinned to
   domains.go:84 ↔ cli_test.go:593). Fix: chunk size injected through
   runScan (the documented test seam); the global became a const. The
   race is now unrepresentable, not merely unscheduled.
7. **T13 go-output polish — core landed (uncommitted at report time)**:
   - `--color auto|always|never` on scan/replay/show (invalid → exit 2).
   - Summary footer ("N domains | M workspace | K gcp") for table and
     markdown ONLY — verified first that go-output's csv/tsv renderers
     write footers as DATA rows, so delimited formats stay pure for
     machine consumption.
   - AddRowChecked everywhere (column-count drift now errors, not garbles).
   - Title plumbing through render.TableOptions.
   - Live smoke verified: ANSI colors render, footer renders, verdicts
     correct (artmann.tech certain+certain; nope.invalid nxdomain).
   - render+cli tests green; full-suite `-race` re-run pending after the
     remaining T13 test additions.

## b) PARTIALLY DONE

1. **T13 remainder**: footer/color/flag tests (plan rows 13.4), runs
   table polish (footer/title there too), README mention of --color.
   Code compiles and is smoke-verified; tests not yet written.
2. **T5 bookkeeping**: the G3 verdict exists only in this report — the
   CHANGELOG entry and the TODO_LIST strike-through are NOT yet written
   (interrupted by this status request).
3. **T17 docs upkeep**: zone-data done; TODO_LIST still carries stale
   rows ("DENIC RDS", "Nominet DAA" naming), the 06-09 toolship §f
   harvest is pending, docs/status index in README pending, `go mod
   tidy` sweep pending.
4. **T19 crt.sh extractor**: not started (zone paperwork IS done — see a4).

## c) NOT STARTED (plan v2 remainder)

- T6: release v0.1.0 (go-release skill; G2+G3 both satisfied — G3 this
  session, G2 by the prior session's T1 decision).
- T14: README honest JSON-claim fix + install section + Homebrew tap.
- T15: WithDoctorCheck preflight (resolver + ranges feed).
- T16: nom live progress for long scans (TTY-only).
- T18: audit upkeep + go-cqrs-lite re-audit + verified-at-commit headers.
- T19 code half: crt.sh fallback extractor (+ now also the .de fallback
  role after the DENIC dead end).
- T20: lint cleanup to zero (err113 ×many, wrapcheck, varnamelen,
  tagalign, cyclop ×2, gocognit, makezero... ~70 warnings in editor).
- T21: cmdguard-lint CI step.
- T22: decided AGAINST (T1 verdict; stays closed).
- T23: registry-derived --format enum.
- T24: NXDOMAIN/A-record pre-sweep (gated T9+T19+G4 — all now clear).
- T25: CSV enrichment mode.
- T26: scoped exit codes in internal/ranges (75 fetch / 65 corrupt).
- T27: infra long-tail (TimeoutMiddleware, describe contract test,
  vendorHash.nix, freshness guard, docs-gates cache investigation, CAA
  watch → ROADMAP).
- Census plan (separate, docs/planning/2026-11-10_06-18_*): untouched by
  me — its T02/T05/T07-T16 belong to the toolship/v2 track.

## d) TOTALLY FUCKED UP (this session's scars, honestly)

1. **I started editing before re-reading daemon-swept files** — the
   tag-alignment format sweep had reflowed flag structs; my multiedits
   landed 2 syntactically-broken struct tags (vet caught both), and 3 of
   7 planned edits missed on whitespace. Cost: ~4 repair rounds that a
   10-second `view` before `multiedit` would have avoided.
2. **Left the tree broken across user interruptions — TWICE**. The T13
   refactor was mid-signature-change (render.Table 3→4 args, callers
   half-updated, tests broken) when the status order arrived. I chose to
   finish-to-green before reporting (right call), but I should have made
   the change ATOMIC: one file's signature + all callers + tests in a
   single pass, instead of a sprawl I couldn't pause cleanly.
3. **Daemon beat my T12 commit by seconds — twice in one task** (my
   `git add` window). New files (show.go, runs.go, queries.go, tests)
   live inside daemon "auto-commit" bundles; only the final amend carries
   the real message. Recoverable, but the history is uglier than the
   per-task commits the plan prescribed. Mitigation for next time: write
   files → `git add` IMMEDIATELY in the same command.
4. **Queried the wrong assumption once**: I initially "fixed" show.go's
   `renderFilterAll` (a name I had invented in my own draft) — a
   self-inflicted compile error from writing code faster than reading it
   back.
5. **Carried a stale mental model into T5**: I expected the 01:49
   session's job `13A` to still exist. Jobs, /tmp artifacts, and session
   state do not survive; only repo state does. The status context said
   so; I checked /tmp first anyway (cheap, but the expectation was wrong).

## e) WHAT WE SHOULD IMPROVE (process)

1. **Atomic refactors only**: signature changes land with ALL callers +
   tests in one edit burst; never leave a half-migrated call graph
   reachable by an interruption.
2. **`view` before `multiedit` in daemon-swept repos** — the format
   daemon reflows whitespace between reads; my edit cache is only valid
   until the next daemon pass.
3. **`git add` in the same shell line as writing new files** — collapses
   the daemon's race window on untracked files.
4. **T5 pattern is now the template for bulk runs**: pilot at high
   concurrency → decompose errors by kind → re-sweep the transient tail
   at low concurrency → re-verify the nxdomain bucket against a clean
   resolver before quoting coverage numbers. This should become docs
   (README bulk-scan section) and eventually tooling (a `--retry-errors`
   that queries the journal).
5. **False-NXDOMAIN awareness**: ~10% of nxdomain verdicts under a
   stressed resolver were wrong in this run. Any consumer treating
   nxdomain as ground truth (pre-sweep T24 especially!) must re-verify
   against a second resolver. Pin this in T24's design notes.
6. **Editor LSP cache is stale** (still reports counts.go:47 ErrKind
   undefined and scanChunkSize-as-global; both are fixed in code). Stop
   trusting project diagnostics from the editor; trust build/vet/test.
7. **Mirror hygiene**: both remotes are ~20+ commits behind local. The
   split-brain window grows every hour; push policy needs a decision (see
   g1).

## f) Next — up to 50, impact-ordered

**Ship the release (spine completion):**

1. Record T5 G3 verdict in TODO_LIST + CHANGELOG (10 min).
2. Finish T13 tests: footer table/markdown, csv/tsv footer-free, --color
   parse errors exit 2, --color never (no ANSI) (25 min).
3. runs table polish: footer + Title (10 min).
4. T6: v0.1.0 release via go-release skill (CHANGELOG cut, tag, push
   both remotes, GitHub release, proxy check, rollback note) (60-90 min).
5. Push ALL pending commits to origin + github before tagging (g1).
6. T14: README — fix the "one line per domain" JSON claim, add install
   section (go install + nix), document --color/--resume/show/runs/
   --event-store, bulk-run guidance with the T5 lessons (45 min).
7. Homebrew tap entry for the release (T14 second half).
8. Watch the first real CI run on the pushed commits; fix findings.

**Product depth (sellable output):**
9. T25 CSV enrichment mode (append verdict columns, preserve input cols).
10. T23 registry-derived --format enum (drift-proof vs go-output).
11. T15 doctor preflight (resolver reachability + ranges feed check).
12. T16 nom live progress (TTY-only) for long scans.
13. `--retry-errors <run>`: journal-driven error-tail re-sweep (my T5
manual workflow, productized; needs the same linked-run semantics as
--resume).
14. Delta query: verdict flips between runs (census T16; the killer
prospecting signal).
15. Exit-code scoping in internal/ranges (T26: 75 fetch / 65 corrupt).

**Scale readiness (G4 → whole-TLD):**
16. T24 pre-sweep design doc — MUST incorporate the false-NXDOMAIN
finding (re-verify pass before dropping domains).
17. Unbound local resolver spike (census T05) — kills the throttle wall
the T5 run hit at 24 workers.
18. Recover the 7,204 legacy unknowns via unbound + error-tail files.
19. Throughput benchmark + qps politeness budget doc (census T08).
20. Pilot harness slice + error-rate gate as documented pre-flight.
21. .ch zone AXFR dry-run against zonedata.switch.ch (keys are public;
once/24h; verify the ~2.4M-name extraction path end-to-end).

**Zone paperwork (owner actions):**
22. Send the Nominet .uk application email (draft ready in
docs/zone-data/nominet-uk.md) — weeks of lead time.
~~23. Decide the .ch research-purpose question (public-interest clause vs~~ done — moot — SWITCH zone access is public TSIG (no research-clause paperwork); the pull landed 2026-11-13 (2.66M apexes)
commercial lead-gen use) before the first .ch census wave.
24. Drop/rewrite the stale DENIC rows in TODO_LIST (research landed:
impossible).

**Quality gates:**
25. T20 lint sweep to zero (~70 warnings; err113/wrapcheck/varnamelen/
tagalign/cyclop/gocognit/makezero/nilnil/golines).
~~26. T21 cmdguard-lint CI step.~~ done — cmdguard-lint CI step
27. Full `-race` suite re-run after T13 tests (the pre-T13 run was green).
28. buildflow gate run (blocked by BuildFlow#56 toolchain skew — the
other session owns it; keep --no-verify + direct verification).
29. T18: refresh docs/review/* dashboards with verified-at-commit
headers; re-audit go-cqrs-lite verdict now that it's a direct dep.
~~30. cli coverage 69% → 90 (toolship §f31).~~ done — 90.7% (2026-11-13 batch, ~15 new tests, 3 real bugs fixed)

**Docs upkeep (T17 finish):**
31. TODO_LIST sync: strike T5/T7/T8/T9/T11/T12 + stale DENIC/Nominet
rows; harvest the 06-09 toolship §f and this report's f.
32. FEATURES.md refresh (event sourcing FULLY_FUNCTIONAL: resume, show,
runs, streaming, retry, taxonomy).
33. docs/status index in README (discoverability).
34. `go mod tidy` sweep (post-stabilization).
35. docs-gates green pass.
36. AGENTS.md architecture map: add show/runs + queries.go + feeder
(v2 session's files unreviewed by the 06-09 session; mine are).
37. ROADMAP.md: park CAA watch, ETag-if-polled, M365 detection ideas.
38. VendorHash refresh after any dependency change (nix-hash-fix).

**Robustness / verification:**
~~39. Fuzz the input parser (CSV quoting/encoding edges).~~ done — internal/input/fuzz_test.go (seed corpus incl. CSV quoting/encoding edges)
40. describe contract test (pin output shape post-release).
41. IPv6 resolver-address handling check (--resolver only IPv4-tested).
42. Resolver-dependency bias check (8.8.8.8 trickle results vs Quad9).
43. TXT truncation investigation (cloudflare "cannot unmarshal" class).
~~44. TimeoutMiddleware whole-run budget (T27).~~ done — `--max-runtime` (plan v3 E31)
~~45. vendorHash.nix extraction (T27).~~ **Won't implement — resolved-as-deliberate-non-fix — buildflow's nix-hash-fix owns hash updates; a separate vendorHash.nix adds indirection with zero gain (2026-11-10).**
46. Version-freshness guard for audit reports (T27).
47. GPG signing policy decision for agent commits (toolship §f44).

**Coordination:**
48. Merge the census plan's task numbering with plan v2's under one
TODO_LIST view (two T-numbering systems coexist now).
49. Decide reports/ gitignore vs README pointer (data-not-code policy).
50. Session handoff note for the v2/toolship session: T12 landed on top
of their scanstore work (additive only), chunk-size seam moved.

## g) Questions I cannot answer myself

1. **Push now or hold for the tag?** ~20+ local commits sit unpushed on
   BOTH remotes (mirror split-brain grows; AGENTS.md says "push to
   both"). Push everything now and per-task after, or hold everything
   for the v0.1.0 tag so the first public history arrives curated?
2. **Tag v0.1.0 immediately?** G2 (pre-tag decisions) and G3
   (validation clean) are both satisfied as of this session; the
   remaining pre-tag item is only the T5 CHANGELOG entry (~10 min).
   Proceed straight through the go-release checklist, or do you want to
   review this report / the T5 artifacts under reports/ first?
3. **Scope check for `--retry-errors`:** T5 showed the error-tail
   re-sweep is a MANUAL multi-step (extract tail file → rescan → merge
   mental model), and `--resume` deliberately skips errored observations.
   Productize it now as `scan --retry-errors <run|latest>` (new run
   linked like --resume, ~60-90 min incl. tests), or keep the manual
   workflow and put it in the census plan's unknown-cleanup loop (T14
   there) where it is already implicitly planned?

---

_Point-of-time snapshot; annotate, never rewrite. Written 2026-11-10
07:46 CET after `go build`/`go vet` green + render/cli tests green +
T5 artifacts intact under `reports/2026-11-10-t5-validation/`
(scan/replay/tail-sweep/nx-reverify JSON + stderr, t5.db journal, binary).
Working tree at report time: T13 changes present and compiling, uncommitted
(daemon will sweep them); HEAD = 80c1c60 (T12)._
