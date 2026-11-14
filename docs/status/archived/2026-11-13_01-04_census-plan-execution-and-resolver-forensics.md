> **Live remainder:** every item resolved as of the 2026-11-13 docs-health pass (docs-gates green); open census work lives in TODO_LIST.md + docs/census.md.

# Status Report — Census plan execution: scanstore review, ruleset stamps, resolver forensics

> **2026-11-13 resolution (post-fix annotation):** this report's central
> forensic claim — that the false NXDOMAINs were the _forwarder
> fabricating wrong authoritative answers under burst load_ — is
> **FALSIFIED**. The root cause was scanner-side: Go's resolver maps both
> NXDOMAIN and NODATA ("name exists, no such record") to the same
> IsNotFound error, and the scanner treated it as domain death. The
> falsifying experiment (same 6 false-NX verdicts reproduced through a
> fresh-cache TCP-upstream instance in isolation, then all 6 clean after
> the existence-oracle fix) and the fix are in `docs/census.md` ("false-
> NXDOMAIN root cause") and the existence-oracle session report. The
> "transient, resolves via unbound minutes later" observation was a
> misreading: the sampled digs queried A records, the scanner's fatal
> lookup was MX — the discrepancy was deterministic, not transient.
> Wave A's 22,010-domain NX bucket was ~90% false and is being
> superseded by the tranco1m-a-nxfix correction wave.

**Written:** 2026-11-13 01:04 CET · **Session scope:** execute the
2026-11-10 06:18 Tranco top-1M census plan (T01–T29) in a repo where
parallel sessions were actively shipping the same plan. This session
became, in practice: adopt + verify the siblings' census machinery,
land the provenance stamps, then forensics on a poisoned pilot gate.

**Scope line — what this report verified, and how:**

- Verified by fresh command: `go build ./...`, `go vet ./...` clean;
  full `go test -race ./...` green 9/9 packages at the merged tip of my
  T02 work + the siblings' delta/query/lint work; `nix build` exit 0;
  wave manifests `sha256sum -c MANIFEST.sha256` all three OK; pilot
  scan executed live (run `01M74VNNCJS653J8TNR0000001`, exit 0) and
  gate verdict FAIL reproduced twice; false-NXDOMAIN diagnosis by
  direct `dig @9.9.9.9` cross-checks (12/12 sampled NXDOMAINs were
  false); journal inspected via `replay --format jsonl` stderr.
- NOT verified: whether my census-slice fix and the parallel sessions'
  newest commits (c76b183 tip) have green CI (their §b1 says three
  commits unpushed at their report time; I did not push or check gh
  runs); the TCP-only unbound experiment (port 5354) never came up —
  my start attempt died with the shell (connection refused), and the
  polish-lane session has since shipped its own `unbound-tcp.conf`
  fallback path, superseding my experiment.

## a) FULLY DONE

1. **T01 — scanstore session review, adopted as canonical.** Read
   events.go / scanstore.go / replay.go / views.go / cli wiring end to
   end. Verdict: correct ES discipline (verdicts derived at read time,
   never stored; per-domain streams; run linking via scan.started
   ResumedFrom; replay boots WITHOUT the projection host; bounded
   SeekableJournal walks; Sink = buffered channel, sequential writes,
   3-attempt backoff, drop-count on failure). Full `-race` suite green
   (scanstore 567s cold). No changes needed; AGENTS invariants held.
2. **T02 — provenance stamps (my plan's guard 1+2), LIVE IN
   PRODUCTION.** `classify.Ruleset` type + `CurrentRuleset =
   "v1-2026-11-10"` (ruleset.go + golden-pin test), `Wave` +
   `Ruleset` on ScanStartedPayload, `Ruleset` on every
   DomainObservedPayload (additive, omitempty — pre-stamp journals
   decode fine, pinned by TestPreProvenanceJournalStillReplays),
   `Recorder.StartScan(... wave ...)`, scan `--wave` flag (usage-error
   without --event-store), replay summary `[wave X, ruleset Y]`
   suffix, `RunSummary` carries Wave/Ruleset. Tests: provenance stamps
   roundtrip through real sqlite + legacy-payload decode. The
   interrupted wave-A run in census.db carries the stamps (verified in
   replay stderr). Parallel sessions later verified this as "E11".
3. **T03/T04 — coordination + docs truth pass.** TODO_LIST: struck
   done rows with evidence (batch retry → retryResolver 49b87fc; jsonl
   streaming; journal --resume 3674285; error taxonomy; sink retry;
   CLI hardening incl. validate/ExitError(2)/env prefix/recovery;
   memory row downgraded to buffered-format guidance) and added the
   census section with an explicit ownership note (sibling v2-journal
   plan canonical for scanstore/CLI internals; census plan owns
   waves/manifests/reports/deltas). STACK/AGENTS rows already present
   from the siblings.
4. **Resolver forensics — false NXDOMAIN discovery (the session's key
   result).** The 5k pilot gate FAILED (12.58% unknown vs 2% budget)
   with 18.92% "nxdomain, domain truth". Sampling 12 NXDOMAINs and
   querying Quad9 directly: **12/12 false** (herokuapp.com, ustream.tv,
   fortinet.net, 10086.cn, blogspot.de … all NOERROR direct). The
   false answers were transient — the same domains resolve through
   unbound minutes later (negative-cache expiry), so the burst-window
   forwarder path returns WRONG authoritative answers under load, not
   just timeouts. This is the worst census error class (fake "domain
   truth" would permanently pollute the census), and it independently
   explains the sibling's interrupted wave-A mass-cancellation run
   (93,758 canceled / 487 suspect NXDOMAIN). The polish-lane session's
   later "lying-NXDOMAIN finding" and UDP:53-outage incident confirm
   the pathology from a different angle.
5. **census-slice.sh pipefail bug FIXED.** `tr | head` under
   `set -o pipefail`: head exits after N lines → tr SIGPIPEs → script
   exits 141 on SUCCESS. Reordered `head | tr` (cheap filter first);
   verified exit 0 with correct 5000-line output. This would have
   failed every future gate invocation.
6. **Orphan resolver killed.** My own T05 spike (nohup-in-killed-shell)
   left a second unbound in FULL-RECURSION mode SO_REUSEPORT-sharing
   port 5353 with the canonical forwarder — stealing queries into the
   measured-to-collapse recursion path and very likely contributing to
   the interrupted wave-A run. Killed (only canonical instance
   remains, verified by pgrep + dig). Session lesson recorded in §d.
7. **Census hygiene for the next launch.** Wave manifests
   sha256-verified (all OK); census.db backed up CONSISTENTLY via
   `sqlite3 .backup` (integrity_check ok) after catching that a raw
   file copy misses the 4 MB WAL; backup:
   reports/census/census.db.pre-waveA-pass2.bak.

## b) PARTIALLY DONE

1. **Wave A (per the 2026-11-12 18-45 sibling report, newer than this
   session's pilot): COMPLETE — health PASS, unknown 1.53%; the
   2,731-domain resume hole root-caused and gated shut.** Still open:
   the post-outage cleanup round (13 00:03 report) and a verified
   re-scan of my pilot's 946 poisoned NXDOMAINs (append-only journal;
   correction = later healthy runs overriding via replay last-wins,
   plus the nxdomain-bucket re-verify row). Wave B is HELD pending
   that cleanup.
2. **T05 unbound spike:** resolver infrastructure exists and is
   measured (sibling E7/E9); my spike's only lasting value was
   negative (the orphan). TCP-fallback unbound-tcp.conf now exists
   from the polish lane — unverified by me.
3. **Waves B/C, census report (T15), enrichment/lead-scoring
   (T18/T20):** untouched — now gated on wave A completing clean.

## c) NOT STARTED (from my plan, still open)

- T12/T13 waves B and C (gated on a clean wave A + healthy resolver).
- T15 journal-derived census report + archive; T16 daily delta timer;
  T19 journal-only stats parity; T20 lead scoring (delta query itself
  shipped: `replay --since`).
- T22 cli coverage 90%; version-freshness guard script; E22 re-audit
  citations; BuildFlow#56 fleet fix (owner approval gate).

## d) TOTALLY FUCKED UP

1. **My orphan resolver poisoned the shared port.** I ran a spike
   unbound (full-recursion, the measured-to-collapse mode) on the SAME
   port as the canonical census forwarder, detached it with nohup, and
   my job_kill killed the shell but not the daemonized child. It
   likely contributed to the sibling's interrupted wave-A run. Root
   failures: (a) ignored my own plan's guard 5 in spirit (user-scoped
   YES, but also "the census resolver is shared infrastructure — never
   run a second one"); (b) used a hand-rolled nohup while
   `scripts/unbound.sh` lifecycle existed; (c) kill didn't verify the
   child was gone.
2. **Gate-after-write ordering.** I launched the pilot scan writing
   into census.db BEFORE reading its gate — a FAIL then polluted the
   journal with 946 poisoned observations. Gate-first (or
   scan-to-/tmp) costs nothing.
3. **Raw `cp` of a live SQLite journal.** The first backup missed the
   WAL (4 MB of committed data outside the main file). Caught it by
   checking for -wal/-shm and redid it with `sqlite3 .backup` — but
   the first attempt would have been a silently corrupt restore point.
4. **Trusted the tool's exit code over its semantics, once.** The
   slice script's 141-on-success bug (now fixed) had been silently
   making every caller treat success as failure — and my first pilot
   invocation chained on it and produced misleading "scan-exit=141"
   output before I read the script.
5. **Started work before checking for live parallel sessions.** I
   implemented plan items the siblings had already shipped (retry,
   resume, delta, manifests) and had to re-survey twice. The mixed
   auto-commit history from daemon sweeps made ownership archaeology
   harder (their §d2 complaint, mirrored).

## e) WHAT WE SHOULD IMPROVE

1. **Nxdomain trust requires verification, always, at census scale.**
   Any pass whose NXDOMAIN fraction exceeds its historical band must
   trigger sample re-verification against a second path (direct dig)
   before the data is called "domain truth". The gate should grow a
   `--verify-nxdomain N` mode (the polish lane's distress detection
   partially covers this).
2. **Shared-infrastructure processes need lifecycle ownership.** One
   resolver, one launcher (scripts/unbound.sh), pidfile + health check
   before every wave; anything else on the port is an incident.
3. **Before executing a plan in this repo, diff the plan against
   `git log --since` first** — sibling sessions move faster than the
   plan doc ages.
4. **sqlite backups of live journals are `.backup` or nothing.**
5. **Detached long jobs: start via the lifecycle script or a
   supervisor, never bare nohup** — and always verify the child
   survives the parent before declaring the job started.

## f) Up to 50 next things (ranked; items the sibling lane already

owns are marked)

1. Post-outage cleanup round for wave A + nxdomain-bucket re-verify
   (sibling lane, in their list).
2. ~~Fresh 5k pilot gate through the TCP-fallback config; only PASS~~ done at `ae05f57` (wave B launched on the TCP-validated path, 2026-11-13)
   launches the next wave (B, post-cleanup).
3. T15 census report (journal-derived, per-TLD, coverage table,
   archived under reports/census/ + CHANGELOG row).
4. Journal-integrity pass: flag runs whose NXDOMAIN fraction is
   anomalous (pilot 01M74VNN…, interrupted 01M74QS4…) so reports
   exclude or re-scan them.
5. Gate: `--verify-nxdomain N` sampling mode (dig @9.9.9.9 cross-check
   inside census-gate.sh).
6. Push the 3 unpushed commits + watch first CI run (sibling lane).
7. ~~T22 cli coverage → 90%.~~ done at `f169854` (90.7% cli coverage, E33)
~~8. audit-freshness.sh (version-pin guard).~~ done — scripts/audit-freshness.sh + CI lib-freshness job
9. T17 daily delta timer; T19 journal-only stats; T20 lead scoring.
10. E22 re-audit: go-cqrs-lite + cqrs-htmx premise flip.
11. BuildFlow#56 fleet rebuild (owner approval).
12. FEATURES.md refresh (census tooling, delta, tap, soak).
13. Post-census journal-compaction decision.

## g) Questions I cannot answer myself

1. Should the poisoned pilot run (01M74VNN…) and the interrupted run
   (01M74QS4…) be (a) left in the journal and excluded by report
   filters, or (b) superseded by full re-scans of their domains in the
   next healthy pass (replay last-wins already prefers newer runs)?
   (a) preserves history, (b) makes `replay` output directly truthful
   without filters.
2. Is TCP-only forwarding acceptable as the census default given its
   latency cost (it trades wall-clock days for answer integrity), or
   should UDP remain default with TCP only as the verified fallback?
3. The host's outbound UDP:53 path degraded twice within 24h (burst
   false-NXDOMAINs + the 23:24 outage). Before burning ~3 days of
   waves: is a network-path change (wired uplink, different host, or
   VPN egress) an option the owner wants evaluated first?
