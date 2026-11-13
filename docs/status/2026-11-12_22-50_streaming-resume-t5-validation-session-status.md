# Status Report — Streaming/Resume Session + T5 Validation (interrupted; superseded by plan v3)

**Written:** 2026-11-12 22:50 CET · **Session covered:** the 2026-11-10
~01:52–02:20 continuation of the "WHOLE plan v2" execution (T7, T8, T9,
T11-partial, T5 analysis) — interrupted mid-verification; **re-checked today
2026-11-12 22:50** against the tree two days of later sessions built on top.

## Scope line — verified vs not verified

**Verified today, with commands:**

- `go build ./...` exit 0; `go test ./... -count=1 -short` — all 8 packages
  ok (scanstore 0.39s incl. the streaming tests).
- `git status -sb` → `## master...origin/master`, clean and NOT ahead:
  everything this session touched is committed AND pushed (by later
  sessions).
- `git tag` → **v0.1.0 exists** (T6 done after this session).
- Artifacts of later sessions visible: `scripts/` census suite
  (census-wave/gate/snapshot/slice/cleanup, crtsh-extract, unbound),
  `reports/census/` (db, snapshots, waves, unbound, tranco), plan v3
  (`docs/planning/2026-11-10_09-21_unified-execution-v3-ship-then-census.md`).

**NOT verified / lost:**

- The clean T5 rerun (launched 2026-11-10 ~02:10, `--resolver 9.9.9.9`)
  and the before/after memory A/B (`/tmp/gdd` old binary on the 50k list):
  background jobs died with the session restart; `/tmp/t5-clean.*` and
  `/tmp/t7-50k-before.*` are gone. Both are superseded (see c).
- The dig ground-truth was sampled (14/15 + 10 verdicts) but the full 506
  "nxdomain" bucket was never re-classified; the census (later sessions)
  re-approached this properly with the unbound forwarder.

---

## a) FULLY DONE (this session's run; all green at the time and re-verified green today)

1. **T7 — jsonl streaming scan output.** `render.JSONLStream`
   (mutex-serialized, first error sticks, completion-order lines);
   `chainOnResult` in cli preserves the event-store sink while adding the
   stream; `--only` filter applied per streamed line. Tests:
   slow-domain-lands-LAST (proves streaming vs buffering) and
   cancel-mid-stream (completed domain keeps its line, hung domain reports
   `canceled`). Semantic finding pinned: a parent _deadline_ expiry
   classifies as `timeout` (indistinguishable from per-domain timeout —
   correct); real Ctrl-C (explicit cancel) reports `canceled`.
2. **T8 — streaming end to end.** `input.Streamer` (lazy plain/CSV mode
   detection on the first non-empty line, exact skip counts capped at 5
   kept examples); `domainFeeder` (chunked, cross-source first-seen dedup,
   per-source file closing, `scanChunkSize` var for boundary tests);
   `runScan` chunk loop (per-chunk `Counts.Tally`, buffered formats
   unchanged, jsonl discards per chunk); scanstore `walkJournal` (batched,
   bounded memory) + `StreamObservations` + `ErrNoRuns` sentinel; `replay
   --format jsonl` streams from the journal walk. Tests: input parity
   (args/file/stdin identical output), multi-chunk scan (3 chunks) records
   and replays exactly, StreamObservations ≡ ReplayJournal, empty-store
   sentinel, 10k-event store streams 10k/10k in 1.6s.
3. **50k-domain streaming smoke (T7.7).** EXIT 0, wall 28 min (DNS-bound),
   **MAXRSS 40 MiB flat** — the memory claim for jsonl streaming held at
   50k scale. Error tail came out fully classified (49 525 nxdomain /
   475 timeout) — though the DATA was later found resolver-poisoned (see d).
4. **T5 first validation run — analyzed.** 3 645 `.ch` domains: workspace
   certain 304 / likely 134, gcp 41, errors 1 625 (506 nxdomain, 989
   timeout, 130 servfail). **Replay-vs-live content diff: 0/3 645** (sets
   compared, order differs by design). **10 dig cross-checks:** 3
   workspace-certain (aspmx.l.google.com MX ×3), 2 likely (SPF
   include:_spf.google.com ×2), 1 gcp-certain (bader.ch A
   34.144.248.61 ∈ 34.144.192.0/18 cloud.json — re-confirmed live), 2
   negatives correct, 2 recorded-NXDOMAINs **disputed** → triggered the
   investigation below.
5. **Critical finding — a distressed local resolver LIES NXDOMAIN.**
   127.0.0.1 under concurrent bulk load (two orphaned scan processes of the
   dead 01:49 session + the in-flight T5 run) returned NXDOMAIN for live
   domains: **14/14 sampled recorded-nxdomain domains were ALIVE** at
   9.9.9.9/8.8.8.8 (own NS delegation, A records). The entire 1 625-error
   tail of run 1 was resolver distress, not domain state. Zombies killed
   (`pkill`), clean rerun launched via `--resolver 9.9.9.9` (result lost to
   the restart; superseded by census). This finding is now codified in
   AGENTS.md's "distressed resolver" gotcha (written by the census
   sessions, which hit the same wall and fixed it structurally with the
   unbound forwarder).
6. **T9 — journal-based `--resume`.** `scanstore.LoadResumeSource`
   (exact run ID or `latest`, validates existence, key-set-only memory);
   feeder skip-set; additive `scan.started.ResumedFrom` payload +
   `RunSummary.ResumedFrom`; "all domains already observed → nothing to do"
   no-op UX; `--resume` requires `--event-store` (usage error 2). Tests:
   resume skips observed domains and links back to the source run; resume
   of an interrupted (started, uncompleted) run works. Committed with a
   proper message (`3674285`) — the one commit this session won against
   the daemon.
7. **T11 core (Sink hardening) — code landed.** Bounded per-observation
   retry (3 attempts, 25 ms→50 ms backoff), loud Close error carrying the
   exact dropped count ("N observations dropped"), `runRecorder` seam so
   tests can inject store failures, `CompleteScan` on
   `context.WithoutCancel` (a Ctrl-C'd scan still closes its run record).

## b) PARTIALLY DONE (at the interrupt)

1. **T11 tests + verification.** Sink retry tests (transient recovers /
   permanent fails loudly with count) and the 100k-event journal benchmark
   were written, but my scripted python import surgery mangled
   `scanstore_test.go`'s import block twice (id/v4, command/v4); the last
   repair was never compile-verified in-session. **Resolution (verified
   today):** later sessions carried it over — the G4 verdict in AGENTS.md
   cites `TestJournalOneHundredThousandEvents` (100k: append 18.7 s /
   5 345 events/s, replay 2.0 s / 51 035/s, db 125 MiB, opt-in
   `GDD_SOAK=1`), and the suite is green.
2. **Clean T5 rerun + before/after memory A/B.** Launched, never observed
   (jobs + /tmp artifacts lost on restart).
3. **T11.6 (G4 verdict into AGENTS.md).** Not done by me — done by the
   later session (see above).

## c) NOT STARTED (by this session)

T6 release, T12 show/runs, T13–T27. **Status today per the current tree
(verified): T6 DONE** — v0.1.0 tagged, CHANGELOG cut, origin in sync;
**T12 DONE** (`internal/cli/show.go` in the architecture map, de-ghost
note in AGENTS.md); **T19.4 DONE** (`scripts/crtsh-extract.sh` + zone-data
docs); census infrastructure (unbound forwarder 127.0.0.1:5353, wave/gate/
snapshot scripts, single-flight law, provenance stamping) built and
**census wave A (100k) complete** at 93%+ coverage with the 2 731-domain
`--resume latest` hole root-caused and gated (exit-4 manifest check).
Still open from the v2/v3 stack (36 TODO rows): waves B/C, unknown-cleanup
loop, census report + delta query (T16-census), resume-skip accounting
print, buffered-format memory note, cmdguard-lint CI, audit upkeep,
tidy sweep, lint-to-zero, FormatsForShape (T23), CSV enrichment (T25),
Nominet DAA paperwork (owner), .ch zone pull via SWITCH TSIG, input-parser
fuzzing (E33), TimeoutMiddleware (E31).

## d) TOTALLY FUCKED UP

1. **Lost the daemon race for T7+T8** — both swept under junk
   auto-commit messages before my `git add`; content safe, messages lost
   (only T9 got a proper commit).
2. **Mangled a test file with scripted sed/python surgery — twice** —
   instead of manual edit-tool fixes; left T11 unverified mid-flight at
   the interrupt. The single worst mechanical mistake of the session.
3. **Called the first T5 run "fully explained" before ground-truthing
   it.** The taxonomy readout was mechanically right but the underlying
   NXDOMAIN data was resolver garbage; the dig sampling that exposed it
   was a later step I could easily have skipped. Validation claims need
   ground truth FIRST.
4. **Created my own resolver single-flight violation:** launched the 50k
   smoke while the T5 run + two zombies were still scanning; later
   launched the T5-clean rerun concurrently with my own A/B job. (The
   census single-flight law of 2026-11-11 now forbids exactly this.)
5. Missed that the conversation env said 11/12 — resumed as if continuous
   11/10; background job IDs (0D9/0E3) became invalid, losing both
   results.
6. One multiedit with inverted old/new strings; one `mustTuple`
   placeholder hack (both caught pre-commit); a `closerReader`
   double-reader file-handle bug (caught pre-commit).
7. LSP died mid-session (`jsonrpc2: connection is closed`) and was never
   restarted — worked compiler-only amid stale typecheck ghosts.

## e) WHAT WE SHOULD IMPROVE

1. **Commit per edit batch, immediately** — the daemon's sweep window is
   seconds; every lost race costs a junk message or recovery surgery.
2. **Session start ritual:** `date`, `pgrep -f 'gdd scan'`, read the
   newest `docs/status/` + AGENTS.md delta BEFORE resuming stale mental
   state; a restart may have landed days of work.
3. **Ground-truth before verdict:** any bulk-run error tail gets a dig
   sample against a public resolver before it is called "explained".
4. **No scripted edits to import blocks** — use the edit tool or
   goimports; naive string surgery garbles grouped imports.
5. **Persist background-run outputs to dated files and read them before
   yielding** — background job IDs do not survive restarts.
6. Restart the LSP when it dies; don't reason around stale diagnostics.

## f) Next things (ranked; from the live TODO_LIST + plan v3, 36 open rows)

1. **T11-patch:** scan the 2 731 wave-A resume-skipped domains (census
   gate exit 4 currently holds the wave closed).
2. Waves B (100–500k) then C (500k–1M) with pilot gates (single-flight
   law, unbound forwarder, `scripts/census-wave.sh`).
3. Unknown-cleanup loop per wave (<0.5% unknown bar).
4. T16-census: delta query — verdict flips between runs (the sales
   signal); retires the CRM script's diff logic.
5. Census report (journal-derived) archived under reports/.
6. Resume-skip accounting on stderr ("N domains skipped as already
   observed in run X").
7. Interrupted-run stderr summary counts synthetic canceled fills — fix
   the double-count.
8. www-strip normalizes `www.<public-suffix>` inputs (26 known cases).
9. Buffered formats (table/json/csv) still materialize — document or
   chunk-render (long-tail).
10. Own-resolver guidance doc for whole-zone scale (unbound/knot).
11. CAA/other record classes — only if a real fingerprint emerges.
12. CSV enrichment mode (input CSV + appended verdict columns).
13. Nominet .uk zone-file licence — OWNER sends the ready draft email.
14. SWITCH .ch zone pull via public TSIG keys + .ch corpus build.
15. crt.sh .de extractor rate-limit-aware scheduling.
16. NXDOMAIN/A-existence pre-sweep before whole-TLD scale.
17. cmdguard-lint as a CI step (expect zero).
18. Audit upkeep: re-verify `docs/review/*` citations vs current HEAD;
    cqrs re-audit post-scanstore.
19. `go mod tidy` sweep post-feature-stabilization.
20. Lint cleanup to zero (err113, wrapcheck, errcheck, tagalign,
    varnamelen, cyclop…).
21. Version-freshness guard for audit reports (pinned-commit staleness).
22. TimeoutMiddleware whole-run budget (plan v3 E31).
23. `output.FormatsForShape`-derived `--format` values (drift-proof).
24. Input-parser fuzzing (CSV quoting/encoding, E33).
25. Retract-note dry run for v0.1.0 (documentation-only procedure).
26. Push posture: keep both remotes in sync at task boundaries (origin
    was 12 commits behind at one point during this session).
27. BuildFlow#56: rebuild buildflow with go ≥1.27.2 to un-pre-commit-hook
    the repo (lives in the BuildFlow repo).
28. Consider `gdd` short-binary alias / install story polish (README tap
    row if still open).
29. Post-v0.1.0: pkg.go.dev propagation spot-check on next release.
30. Keep AGENTS.md census laws current as waves B/C shift operational
    practice.

## g) Questions only the owner can answer

1. **Census vs plan-v2 remainder:** plan v3 (ship-then-census) is clearly
   the active spine — should the remaining plan-v2 polish tier (lint zero,
   cmdguard-lint CI, audit upkeep, FormatsForShape, CSV enrichment) run
   BEFORE waves B/C, or does the census own the machine until Tranco 1M is
   done and polish follows after?
2. **T5 validation closure:** the original G3 gate ("v0.1.0 only after a
   clean validation run with an explained error tail") was satisfied by
   later sessions' work (v0.1.0 is tagged, wave A validated at 100k). Is
   the lost clean 3 645-domain rerun still wanted as an artifact, or is
   wave A the canonical validation of record?
3. **Zone-data paperwork:** the Nominet draft email and the SWITCH TSIG
   key request are owner-action items that have sat ready since 11-10 —
   do you want them re-surfaced/refreshed and staged for sending now, or
   is Tranco+census coverage sufficient and the zone files park until the
   census completes?
