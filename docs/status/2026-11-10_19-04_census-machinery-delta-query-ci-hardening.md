# Status Report — census machinery proven, delta query landed, CI hardened (tree red under sibling edit)

**Written:** 2026-11-10 19:04 CET · **Session scope:** "GET SHIT DONE — the
WHOLE TODO LIST" continuation from the 13-28 handoff. The handoff was
STALE on arrival: a sibling had already tagged v0.1.0 (9308d97, both
remotes, GH prerelease, proxy/pkg.go.dev/go-get verified), recorded E6,
and removed the pre-T7 worktree — this session verified all of that
instead of redoing it, then executed the census-feasibility tier (E7–E12),
E10 on real data, E18 (delta query), E23, E33-fuzz, and the release/CI
tail (f1/f3/f42–f45, tap, soak workflow, E22 headers, TODO truth pass).

**Scope line — verified with commands this session:**

- Full `-race` suite green on the E18 tree (72.6s scanstore, rest fast);
  gofmt clean at every commit; docs-gates 5/5 PASS at commit time.
- Soak workflow dispatch run GREEN (9m13s) — the nightly schedule is real.
- E9 ladder measured live (recursion w2/4/8, forwarder w4/8/16, cold
  caches, fresh-domain isolation probes) — numbers in docs/census.md.
- E10: 7 204 legacy unknowns extracted (count matches the historical
  report exactly), recovery run + cleanup round completed into
  reports/census/census.db; FuzzParse 30s campaign = 4 539 010 execs,
  zero crashes; crt.sh extractor fixture self-test PASS.
- NOT verified: current tree compiles (sibling mid-edit in cli/domains.go
  — §d1); sustained-w8 Quad9 behavior beyond 2h; wave A pilot (blocked
  behind §d1 + §g1).

---

## a) FULLY DONE (this session)

1. **Stale-handoff reconciliation:** release state verified end-to-end
   (tag/remote/CI history/release body), nothing redone, nothing
   duplicated. Release-body known-issue note (§f3 of 13-27) already
   present.
2. **CI hardening (Q1 decided + implemented):** `go test -race -timeout
   20m` in ci.yml; new nightly `Soak` workflow (03:17 UTC, GDD_SOAK=1,
   45m budget, manual dispatch) — dispatch run green in 9m13s.
3. **E7 finished:** lifecycle `scripts/unbound.sh` (start/stop/status/
   restart, user-scoped nix shell), canonical tracked config
   `scripts/unbound.conf`, lab notes `reports/census/unbound/NOTES.md`
   (full investigation timeline).
4. **The unbound root-cause + forwarder redesign (the session's key
   technical result):** tuning threads/outgoing-range did NOT fix the
   ≥4-worker cold choke; isolation tests (victims resolve instantly
   alone, front door responsive mid-burst, mixed-TLD made it WORSE)
   pointed at upstream-flow pressure on the local NAT path. Forwarder
   mode (Quad9) fixed it: w4 = 0.5% unknown, w8 = 1.5% (PASS),
   w16 = 65.5% (ceiling). Wave params: **workers 8, timeout 10s,
   ~4.1 domains/s cold** — full ladder table in docs/census.md.
5. **E8 pilot harness:** `scripts/census-slice.sh` (CRLF-safe),
   `scripts/census-gate.sh` (parses stderr run-summary OR jsonl
   error_kind counts, budget arg; 2% D3 bar / 0.5% cleanup bar),
   gate self-tested against four archived run outputs (correct PASS and
   FAIL verdicts).
6. **E12:** Tranco top-1M frozen (list ID **947N2**) into
   reports/census/waves/tranco1m-{a,b,c}.txt with sha256 manifest;
   splitter `scripts/tranco-manifests.sh` (count/disjointness checks);
   wave ledger in docs/census.md.
7. **E11 verified done** (sibling): Ruleset stamps in scan.started AND
   every domain_observed, provenance tests, replay provenance suffix.
8. **E10 executed on real data:** the 7 204 unknown-class domains
   extracted exactly; recovery scan (wave legacy-2026-11-09-unknowns,
   w8/10s, ~2h) → 3 814 resolved (11 certain / 19 likely / 2 gcp / rest
   plain), 1 665 domain-truth NXDOMAIN; cleanup round 2 (w4/12s) →
   194 more dead, 567 stable SERVFAILs (broken delegations — see §e3),
   **113 timeouts remaining = 1.6% of the original set**.
9. **E18 delta query COMPLETE:** `scanstore.FlipDiff/CountFlips`
   (errored lookups are unknowns, never departures; one-sided domains
   never flip), `replay --since RUN` (every format incl. streaming
   jsonl, adopted/left counts, ruleset-mismatch warning), plus a latent
   bug fix it exposed: `replay --run <unknown>` silently returned an
   empty report — now `scanstore.ErrUnknownRun` fails naming run+store.
   Tests: pure table test, two-run journal fixture, cli e2e (flips,
   counts, streaming, unknown-run). Full -race green. README delta
   section + CHANGELOG + AGENTS architecture row.
10. **E22 (scoped):** all 22 library audit reports pinned with
    "Consumer repo state at review @<commit>" headers; three old reports
    annotated where tokens went generic.
11. **E20 tap:** homebrew-tap formula (source-build from the v0.1.0
    tarball, sha256-pinned, standalone tarball build verified, ruby -c
    clean) pushed to LarsArtmann/homebrew-tap + README listing.
12. **E23 crt.sh extractor:** `scripts/crtsh-extract.sh` (+ fixture
    self-test): wildcards stripped, subdomains collapsed to registrable
    apex, cross-TLD filtered, case-folded; CRTSH_JSON mode keeps the
    rate-limited API out of tests. Wired into docs/census.md.
13. **E33 (fuzz half):** FuzzParse harness (14 seeds incl. NULs, BOM-ish
    bytes, hostile headers, quoted CSV) + on-disk corpus test; 30s
    campaign 4.5M execs, zero crashes. Oracle corrected to the real
    contract (all-invalid input MAY error; bad rows skip).
14. **Docs truth debt (f42–f45):** AGENTS release law (tag only after
    CI green on the pushed commit), -timeout rationale, gh DNS-outage
    bypass note; E1 negative-cache finding prose now tracked at
    docs/review/2026-11-10-e1-validation.md.
15. **TODO truth pass:** 12 rows struck with evidence (release, tap,
    T02/T05/T07/T08/T10, CI-watch, soak decision, unbound tuning,
    ErrNoRuns, vendorHash deliberate-non-fix); T09 + nxdomain-reverify
    moved to in-progress; docs-gates freshness pre-flight run before the
    edit (SAFE); gates 5/5 at commit.
16. **Q2 (D3 tail semantics) decided + recorded** in docs/census.md:
    NXDOMAIN = domain truth; timeout/servfail/canceled/other =
    tool-environment states, excluded from coverage, retried by cleanup
    loops. (Refinement needed — §e3.)

## b) PARTIALLY DONE

1. **E10/T09:** two rounds run; ~113 timeout-class stragglers (1.6%)
   remain — one more trickle round (w2, warm cache) would likely close
   most. Artifacts not yet archived/summarized into a findings README.
2. **E16 (cleanup-loop pattern):** proven twice by hand; not yet scripted
   as a loop-with-bar (`while gate > 0.5%: extract+rescan`).
3. **Wave A + G6 pilot:** blocked behind the broken tree (§d1) and the
   sustained-load question (§g1). Everything needed to launch exists
   (manifest, params, store, gate, runbook).
4. **E33 coverage half:** cli 69% → 90% not started (fuzzing done).
5. **Version-freshness guard (E31.4):** audit pins landed (hard half);
   the comparing script is not written.

## c) NOT STARTED

- Waves A/B/C execution (≈7h/27h/34h wall-clock at measured rate —
  resumable by design); E17 census report; E32 (daily delta timer,
  journal-only stats, lead scoring).
- E24 lint-to-zero policy + mechanical fixes — SIBLING ACTIVELY IN THIS
  LANE (their polish is visible on my fresh E18 code); E25 CI lint steps
  (gated on E24).
- E28 --pre-filter; .ch SWITCH AXFR pull; TimeoutMiddleware; describe
  contract test (blocked by §d1); IPv6 resolver check; TXT truncation
  investigation; asciinema demo; GoReleaser binaries; consumer sweep.
- Sustained-w8 characterization (only the 2h E10 evidence exists).

## d) TOTALLY FUCKED UP

1. **The tree does not compile at report time:** internal/cli/domains.go
   (sibling's in-flight edit — `declared and not used: dom`, five
   `undefined: d`) and internal/scan/scan.go modified. Hit while building
   the describe check. NOT my file, not fixed (G5 lane discipline); if
   the daemon sweeps it, master CI goes red. My uncommitted
   fuzz_test.go sits behind it.
2. **Daemon committed ~4 of my batches under junk messages** (scripts/
   census files, the E18 code split across three chore commits). Content
   verified via --stat every time; one amend per the byte-identical
   policy. Release-history readability degrades session by session.
3. **My census-gate TODO wording genericized hot tokens** ("jsonl",
   "streaming", "input") and unbound three HISTORICAL done-items in old
   reports — docs-gates whack-a-mole, fixed by annotations. Lesson:
   rewording existing rows can shift document frequencies the pre-flight
   does not model (used --duplicates to pass).
4. **First E9 theory was wrong** (TLD-nameserver rate limiting: mixed-TLD
   scored WORSE, 24.25%). Second theory (client budget) was incomplete.
   Root cause found only via isolation + front-door probes. The
   handoff's "healthy at workers 2" belief was also false (3.0% > 2% bar).
5. **Shell `&`-precedence bug** sent an instrumented probe chain entirely
   to background with empty probe-domain — the "front door responsive"
   data was real but accidentally measured (root-NS answers to an empty
   query, not threema.ch MX).
6. **Over-strict fuzz oracle** (Parse must never error) — wrong
   contract; all-invalid input legitimately errors. Caught by my own
   seed before landing.

## e) WHAT WE SHOULD IMPROVE

1. **Commit after EVERY file batch** — the daemon races edits; staging
   before writing the message narrows the window.
2. **`go build ./...` before ANY background launch or doc claim** — the
   tree broke under me mid-session; cheap insurance.
3. **Q2 refinement:** SERVFAIL that is STABLE across rounds through a
   healthy forwarder is domain-side broken delegation (a fact about the
   domain), not tool-environment — count it as explained-dead in
   coverage, unlike burst servfails under distress. The 567 stable
   servfails in E10 round 2 are exactly this class. docs/census.md +
   census-gate.sh deserve a "servfail-stable" note.
4. **Instrumented runs:** no `&` in one-liners (use run_in_background);
   always write logs to files first, analyze from files.
5. **AGENTS.md gaps found:** jq-on-this-host shim breaks relative paths
   and does not auto-iterate arrays (bit me three times); census pointer
   row (forwarder + wave params → docs/census.md). Neither added yet.
6. **Daemon-swept junk commits:** the mixed-commit policy works but is
   reactive; a pre-commit-time `git add <paths>` immediately after each
   edit batch is the proactive half.

## f) Up to 50 next things (ranked)

1. Wait for/verify the sibling's domains.go + scan.go edits compile;
   full -race; push (tree currently red).
2. Commit internal/input/fuzz_test.go (written, uncommitted).
3. E10 trickle round 3 (w2, warm) on the 113 timeout stragglers → close
   T09; archive artifacts + findings README under reports/census/.
4. 30-min sustained w8 soak on fresh wave-head domains — Quad9 throttle
   characterization BEFORE wave A (E10's late timeout spike).
5. G6 pilot: 5k slice of tranco1m-a via census-gate (post-§d1).
6. Launch wave A (`--wave tranco1m-a`, workers 8, 10s, census.db) in
   background; monitor loop (journal growth, hourly gate, cleanup
   trigger).
7. Script the E16 cleanup loop (extract-unknowns → rescan → gate bar).
8. Wave B (gated on A + pilot); 9. Wave C → full census.
9. E17 census report (journal-derived, per-TLD, coverage table).
10. census-gate + docs/census.md: servfail-stable semantics (§e3).
11. describe contract test (31.2) once the tree compiles.
12. scripts/audit-freshness.sh — pinned-vs-go.mod version guard (31.4).
13. E24 .golangci.yml policy layer (tagliatelle snake_case contract,
    err113-in-tests, wrapcheck first-party) once the sibling lane lands.
14. E24 mechanical: errcheck, mnd, varnamelen, cyclop (ScanDomain 15>12),
    exhaustive, paralleltest, modernize, perfsprint, unparam.
15. E25: CI cmdguard-lint + golangci-lint + govulncheck; watch first run.
16. E33: coverage profile → gap list → test batches (69 → 90).
17. TimeoutMiddleware whole-run budget (E31).
18. .ch zone pull via SWITCH TSIG (once/24h) → whole-TLD input.
19. E28 --pre-filter A-existence sweep + volume measurement.
20. E29 CSV enrichment (journal-backed column append).
21. E32 daily delta timer (Tranco refresh + new-domain diff vs journal).
22. E32 journal-only stats; delete the ad-hoc JSON path; parity check.
23. E32 lead scoring v1 (workspace-certain × gcp × Tranco rank).
24. GoReleaser wiring so future releases ship binaries (tap stays
    source-build until then).
25. asciinema/GIF demo for the README (E20 leftover).
26. AGENTS: jq-shim gotcha.
27. AGENTS: census pointer row (params + runbook link).
28. README: link docs/census.md from the bulk/limitations section.
29. E22 full re-audit: go-cqrs-lite + cqrs-htmx (premise flipped).
30. E22: re-verify all 22 audit citations vs HEAD; fix drift.
31. E22: re-screenshot the audit index; href check.
32. IPv6 `--resolver` handling check (IPv4-only tested).
33. TXT truncation investigation (cloudflare "cannot unmarshal" class).
34. docs-gates cache-key investigation + ETag-if-polled (31.5).
35. Nominet DAA email (owner action; draft exists).
36. SWITCH TSIG pull automation script.
37. ccTLD data-use constraint review note (23.4).
38. Watch tonight's 03:17 UTC scheduled Soak (first cron firing).
39. Wave-manifest sha256 verification step before each wave launch.
40. Census db backup cadence before waves (≈1.2 GiB at 1M events).
41. unbound.log rotation for multi-hour waves.
42. mixed-TLD sustained qps measurement → possible w6 sweet spot.
43. Post-census journal-compaction decision (ROADMAP parked item).
44. Consumer propagation sweep for v0.1.0 (likely zero consumers).
45. Retract-note dry run (documentation-only emergency procedure).
46. Coverage re-measure after E24 churn (baseline drift).
47. GPG signing policy for agent commits (owner decision).
48. BuildFlow#56 fleet fix (owner approval gate).
49. FEATURES.md refresh (census tooling, delta query, tap, soak CI).

## g) Questions I cannot answer myself

1. **Sustained-load policy for wave A (7h at w8):** E10's 2h run showed
   late timeout spikes — possibly Quad9 throttling sustained bulk
   clients, possibly just the pathological domain set. Options:
   (a) launch wave A at w8 now and lean on cleanup loops if throttled;
   (b) first run a 30-min w8 soak on fresh wave-head domains to
   characterize; (c) add 1.1.1.1 as a third forwarder for headroom
   (heterogeneous resolver answers). My default: (b), then (a).
2. **The red tree:** cli/domains.go does not compile (sibling mid-edit).
   Default: wait for them to finish, verify, push. If it stays broken
   and the daemon sweeps it (red master CI), do you want me to
   fix-forward inside their lane, or leave it and flag?
3. **Census wall-clock budget:** the full 1M is ~2.9 days of continuous
   scanning at the measured rate (resumable, journaled). Run waves
   continuously through nights (default), or supervised windows only?
