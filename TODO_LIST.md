# TODO List

Short- and mid-term improvement tasks for google-domain-detector. Status
markers: `[ ]` open, `[~]` in progress, `[x]` done.

## Census: Tranco top-1M (plan: docs/planning/2026-11-10_06-18_tranco-1m-census-event-sourced.md)

Ownership (agreed with the concurrent v2-journal plan,
docs/planning/2026-11-10_01-18_*): that plan stays CANONICAL for
scanstore/CLI-internal work (its T-series, plans T3/T4/T9/T10 already
landed); this census plan owns everything downstream of the journal:
waves, manifests, unbound, pilot gates, reports, deltas.

- [x] T02 Evidence-schema version (detection ruleset ID) + wave ID stamped
      into events BEFORE any census wave — done: classify.Ruleset
      (RulesetV1) stamped into scan.started AND every domain_observed,
      provenance pinned by TestProvenanceStamps incl. legacy decode.
- [x] T05 Unbound spike — done: user-scoped lifecycle
      (scripts/unbound.sh), canonical forwarder conf (scripts/unbound.conf),
      E7/E9 worker-pressure notes (docs/census.md,
      reports/census/unbound/NOTES.md).
- [x] T07 Pilot harness — done: scripts/census-slice.sh +
      scripts/census-gate.sh (stderr-summary AND jsonl modes, budget arg)
      + runbook docs/census.md; gate self-tested on archived runs.
- [x] T08 Throughput benchmark — done: E9 ladder (recursion w2/4/8,
      forwarder w4/8/16); wave params: workers 8, timeout 10s, forwarder
      resolver (~4.1 dom/s cold). Table in docs/census.md.
- [x] T09 Recover the 7,204 legacy unknowns (.de/.ch/.uk) — done:
      3 rounds into census.db (waves legacy-2026-11-09-unknowns{,-c1,-c2});
      final: 4,732 resolved, 1,864 dead, 595 broken delegations (D3
      refinement: stable SERVFAIL = explained, gate flag
      `--servfail-stable`), 13 residual timeouts (0.18% < 0.5% bar).
      Findings: reports/census/README.md.
- [x] T10 Tranco full-list import + wave manifests — done: list 947N2
      frozen into reports/census/waves/tranco1m-{a,b,c}.txt (sha256 in
      MANIFEST.sha256), splitter scripts/tranco-manifests.sh, ledger in
      docs/census.md.
- [x] T11 Wave A (ranks 1–100k) — done 2026-11-12 04:51: pass 1 health PASS
      (97,265 scanned, unknown 1.53% < 2%), coverage gap 2,731 resume-skipped
      domains (soak3 poisoning, docs/census.md "Resume hazard"); patch scan
      pending owner go.
- [x] T11-patch Wave-A coverage: scan the 2,731 resume-skipped domains —
      done 2026-11-12/13 (sibling): pass 1 + 5 cleanup rounds, residual
      15/2,731; final twist: the 6 residual "NXDOMAIN" verdicts were
      scanner NODATA-conflation artifacts (3 proven live at Quad9),
      superseded by tranco1m-a-nxfix (see the nxdomain-bucket row below).
- [x] Re-verify the nxdomain bucket via a FRESH-cache resolver — CLOSED
      by root cause 2026-11-13: the bucket was never stale-cache lies —
      Go's resolver conflates NXDOMAIN and NODATA into one IsNotFound
      error, and the scanner treated it as domain death (~90% of wave
      A's 22,010 NX verdicts false; windowsupdate.com-class domains
      alive at Quad9). Fixed: resolve.LookupExistence oracle (rcode
      level) + NODATA-is-absence semantics in scan (commit 7b45a1b,
      binary gdd-20261113, provenance recorded). Correction wave
      tranco1m-a-nxfix re-scans all 22,136 affected domains (disjoint:
      wave A's 22,010 + patch's 126) through the TCP-upstream
      forwarder; post-fix NX verdicts 12/12 confirmed true at Quad9;
      replay last-wins supersedes the false verdicts FOR THE NXFIX
      MANIFEST — the c4/c5 and pilot supersession gaps are the
      T11-poisonfix row below (supersession claims are per-domain;
      proven by replay --audit-excluded, never assumed).
- [ ] T11-poisonfix Correction pass for the poisoned supersessions —
      READY 2026-11-13 (shrunk by the supersession audit from 948 to
      **25 domains** — nxfix superseded the pilot/soak3 pool: pilot 0,
      soak3 0, c4 10, c5 15), awaiting B's single-flight + owner go:
      input list = audit-derived
      `replay --audit-excluded <pilot,soak3,c4,c5>` output
      (reports/census/waves/tranco1m-a-poisonfix.txt). After it runs,
      the same audit must report 0 poisoned before wave-A data is
      declared trustworthy.
- [x] Supersession audit as a tool (e2 of the 08-57 report) — done
      2026-11-13: `replay --audit-excluded ID1,ID2` lists every domain
      whose LAST journal observation comes from an excluded run
      (scanstore.AuditSupersession; all --format renderers; findings
      exit 0 with stderr counts; unknown run IDs fail). Would have
      caught §d2/d3 before docs claimed closure.
- [~] T12–T13 Waves B (100–500k), C (500k–1M) with pilot gates — B
  RUNNING since 2026-11-13 08:45 (gdd-20261113 existence-oracle
  binary, TCP-upstream forwarder 127.0.0.1:5354, w4/10s; gates green
  at launch: nxfix closed 0.42% residual + validated TCP path +
  owner go). C chains after B's gate.
- [ ] T14 Unknown-cleanup loop per wave (<0.5% unknown).
- [~] T15 Census report (journal-derived) archived under reports/ —
      WRITTEN: reports/census/REPORT.md (17-run inventory, excluded runs
      pinned with full IDs + reasons, mechanical supersession audit
      = 25 poisoned domains, hostile-TXT incident, wave tallies).
      Final distribution section lands after wave C + poisonfix close.
- [x] Census gate: manifest-unique-count assertion in scripts/census-gate.sh —
      done: --manifest/--expect-skip coverage check, exit 4 on gap; reproduces
      wave A's 2,731 exactly, would have caught the 90,834 fiction too.
- [x] T16 Delta query: verdict flips between runs (the sales signal) —
      done: `replay --since <run-id>` flip-diff mode (adoption/departure
      signal; errored lookups = unknowns, never departures) +
      `--audit-excluded` supersession audit (internal/scanstore/delta.go).

## Detection coverage

- [ ] Input lane: www-strip normalizes www.<public-suffix> inputs (26 in
      tranco1m-a, e.g. www.edu.cn) to bare public suffixes (edu.cn) and scans
      them; decide skip-vs-scan policy. <!-- fresh:skip sibling-scoped pending owner policy -->
- [x] Scan should print resume-skip accounting ("N domains skipped as
      already observed in run X") so coverage gaps are visible at the
      source, not only at the gate — done: `resume: skipped %d input
      domains already observed in run %s` on the stderr summary
      (internal/cli/cli.go:344). <!-- fresh:skip sibling-scoped pending owner policy -->

- [x] Batch retry (once) for transient resolver failures — done:
      retryResolver in internal/scan (transient only, NXDOMAIN never
      retried), commit 49b87fc.
- [x] Own recursive resolver guidance for whole-zone scale (unbound/knot) —
      done: docs/resolvers.md (throttling ladder, forwarder w4/10s sweet
      spot, TCP-upstream variant for blocked UDP paths, lifecycle scripts,
      verdict-truth law: bulk runs only through a fresh-cache forwarder).
      = census T05.
- [x] `--jsonl` streaming output for very large batches (render results as
      they complete instead of after the whole run) — done: render
      JSONLStream + scan/replay streaming paths.
- [x] Resume/checkpointing for multi-hour runs — done differently: the
      event journal IS the checkpoint (`--resume ID|latest` skips observed
      domains, links runs via scan.started), commit 3674285.
- [x] Consider CAA/other record classes only if a real fingerprint emerges —
      PARKED in ROADMAP.md (CAA watch, T27): no speculative probes in
      classify until a real fingerprint justifies them.
- [x] Optional CSV enrichment mode: emit the input CSV with appended
      workspace/gcp columns, preserving all original columns — done:
      `enrich` subcommand (internal/cli/enrich.go; explicit Flush+Error
      check so buffered-write failures never pass silently). = census T18.

## Whole-TLD coverage (the "all active .de/.ch/.uk" goal)

Tranco top-1M covers the traffic-active subset (~63k of the ~30M registered
domains across the three TLDs). Full zone coverage needs the registry data,
which is paperwork only the account holder can sign:

- [x] .de (~17.5M registered): DENIC RDS — RESOLVED-AS-IMPOSSIBLE: DENIC
      refuses ALL third-party zone access (no program, no fee, no
      contact; FAQ verbatim in docs/zone-data/denic-de.md). .de coverage
      = Tranco + crt.sh fallback only.
- [~] .uk (~10.5M): Nominet zone access — DRAFT READY for owner review
      (docs/zone-data/nominet-uk.md: it's the "Zone Files Access Licence"
      now, terms verified 2026-11-10, free, draft email with [brackets]
      for owner details). Sending the email + MiTek ID verification =
      OWNER action; scanning follows the Files.com credentials.
- [x] .ch (~2.4M): SWITCH zone file — ACTIONABLE NOW: public TSIG keys on
      zonedata.switch.ch, `dig -k ch_zonedata.key @zonedata.switch.ch …
      AXFR ch.` (once/24h; exact command in docs/zone-data/switch-ch.md);
      no owner paperwork. Follow-up task below (harvested 07-46).
- [x] Pre-filter before scanning at that scale — SUPERSEDED 2026-11-13 by
      the in-scan existence oracle: resolve.LookupExistence asks rcode-level
      existence inside the scan itself; a separate pre-filter pass would
      duplicate it. Zone inputs still shrink scan volume (harvest rows
      above).
- [x] Memory: buffered formats (table/json/csv) still materialize the full
      report — fine at 100k domains, not at 30M. Bulk guidance: use
      `--format jsonl` (streams per result; input already chunks at 4096;
      50k-domain smoke = 40 MiB RSS flat). The original "input loads fully
      into RAM" concern is landed-fixed via the streaming feeder.
      Measured A/B table: reports/2026-11-10-e6-memory-ab/README.md.

## Operational

- [x] Release v0.1.0 tag once the first real-world lead list ran clean
      (go-release skill owns the process) — done: annotated tag on
      9308d97, both remotes, GitHub prerelease, proxy + pkg.go.dev +
      scratch-dir go-get all verified (13-27/13-28 status reports).
- [x] Homebrew tap entry (homebrew-tap repo) after the tag — done:
      source-build formula at LarsArtmann/homebrew-tap (dbaa78c, v0.1.0
      tarball sha256-pinned, standalone build verified).
- [x] nix run .#release wiring check — resolved: the flake exposes no
      release app (default/fmt/lint/test only); the release path is tag +
      GitHub Release per the go-release skill. Nothing to wire.
- [x] Interrupted-run stderr summary counts synthetic canceled fills as
      scanned domains — done: the summary now reports real scanned counts
      plus `interrupted: %d input domains were never scanned (canceled
      before their turn)` (internal/cli/cli.go:306); the journal stays
      the truth source.
- [x] Input-parser fuzzing (E33; §f32 of the 08-57 report) — done:
      internal/input/fuzz_test.go (seed corpus incl. CSV quoting/encoding
      edges); fuzzing runs clean.
- [x] cli coverage 69% → 90% (E33; §f33 of the 08-57 report) — done:
      90.7% (2026-11-13 batch, ~15 new tests; exposed 3 real bugs fixed:
      enrich buffered-write swallow, XDG_CACHE_HOME ignored in doctor's
      default cache, UDP/TCP same-port oracle-test race).
- [x] Wave watchdog + heartbeat file (§f16 of the 08-57 report) — done
      2026-11-13: scripts/census-watchdog.sh (EXTERNAL detector: stalled
      run1.jsonl + no completion line → WEDGED=exit 1 / DEAD=exit 2 alarm
      into monitor.log; all three verdicts branch-tested; live on wave B)
      + a stall heartbeat inside census-wave.sh's poller (HEARTBEAT STALL
      log after 2 stagnant 30-min polls), folded in via new-inode install
      so the running wave kept the old code.
- [ ] Triage the 14 cqrs-lint findings handed over at session start
      (10-23 report §c1): A032 on `ResumeSource.RunID` + likely a NEW A032
      on `SupersessionFinding.RunID` (needs the same journal-surface
      ignore comment as `RunSummary`), F031 `ListRuns` unbounded Scan →
      `WithLimit`, C015/C023 ignored Close errors in domains.go; E003/B005
      stay deliberate non-fixes; then re-run `buildflow -s cqrs-lint`.
- [ ] Cross-check the supersession audit against reality (10-23 report
      §c2): re-extract the nxfix-complete snapshot, run
      `replay --audit-excluded <pilot,c4,c5 run IDs>`, expect exactly the
      948 tranco1m-a-poisonfix domains; diff against the list.
- [ ] `runs --format` support (jsonl with FULL run IDs — the truncated
      12-char table forced raw SQL during the poisonfix derivation).
- [ ] AGENTS.md diet: 308 → ≤220 lines (docs/agents-md-size gate
      warning); move census war-stories to census.md.
- [ ] file-size-check debt: 9 files over the go-file-too-large gate
      (cli.go, replay.go, scanstore.go + 6 test files) — split by concern;
      pre-existing at the 2026-11-13 dedup audit (art-dupl zeroed
      separately, see docs/review/2026-11-13-dedup-baseline.md).
- [ ] docs-cadence archive ritual: 29 unarchived reports in docs/status/
      (threshold 15) — verify struck rows, git mv to docs/status/archived/
      with live-remainder banners, re-run docs-gates cadence until green.
- [x] Re-pin vendorHash: buildflow preflight warns stale — done:
      `nix build` green at 5f8feb4 (post-go1.27.4-toolchain pin, no hash
      mismatch; the flake's vendorHash is current).
- [ ] `buildflow format` re-verify after formatter fingerprint drift
      (oxfmt 0.68 → 0.75, dprint 0.64.1 → 0.64.3).
- [ ] Poller: per-rank-band unknown-rate expectations (calibrate on
      wave B's ~2.8% pass-1 vs wave A's 1.53%).
- [ ] `--audit-excluded`: accept wave NAMES in addition to run IDs.
- [ ] io-history-ratchet: `--profile-io` investigation (56 files vs
      median 10 — a memo layer may have stopped eliminating I/O).
- [ ] `incoming/outgoing-num-tcp` contention measurement post-B
      (are 150 ever contended at w8? → NOTES.md).
- [x] Retract-note dry run — documentation-only emergency procedure for
      poisoning a published release note; rehearsed 2026-11-13: both paths
      (A notes-only `gh release edit`, B retract directive + republish)
      rehearsed and written up cold in docs/release-recovery.md.
      go-release lane.

## Library audit follow-ups (2026-11-10, plan: docs/planning/2026-11-10_00-50_*)

- [x] Widen `--format` enum to +yaml,toml,jsonl — done (plan v2 T2,
      commit 4235e02; values tag on scan + replay, render/cli tests, live
      smoke of all three formats).
- [x] Adopt go-atomic-write `WriteWithPerm` in `ranges.writeCache` — done
      (plan v2 T3, commit 417afc7; mtime-TTL trap pinned by test, never
      `WriteIfChanged`).
- [x] CLI hardening: `validate` tag on --workers, `ExitError(2)` for usage
      errors, `WithEnvPrefix("GDD_")`, RecoveryMiddleware — done (ScanFlags
      validate:"min=1,max=512", exitCodeUsage, env prefix + recovery wired
      in cliOptions).
- [x] `--color auto|always|never` via go-output ColorMode — done
      (incl. `--no-color` + NO_COLOR env); SetFooter summary row
      (table/markdown only — csv/tsv verified pure data), Title, AddRowChecked
      everywhere; flag-value rejections now re-tag as usage errors so bad
      `--color` exits 2 (E19 complete: doctor + nom landed 2026-11-10).
- [x] `WithDoctorCheck` preflight (resolver + ranges feed) — done:
      `doctor` carries dns-resolver (5s real lookup, GDD_RESOLVER-aware)
      and ranges-feed (10s load, exit-75/65 families named) checks; failure
      paths pinned offline via swappable probes (internal/cli/doctor.go).
- [x] Decide `--format` vs cmdguard's `-o/--output` BEFORE the v0.1.0 tag
      — done: `--format` kept, `-o` rejected (global flag + CLI-object
      resolution breaks per-command flag structs and the test seam; unix
      `-o` reads as output file). STACK.md decision log, plan v2 T1.
- [x] cmdguard-lint as a CI step (expected zero findings) — done: ci.yml
      `lint` job installs cmdguard-lint from a pinned cmdguard clone
      (`git clone --filter=blob:none` + fixed SHA; the module proxy refuses
      its replace directives) and runs it green.
- [ ] Audit upkeep after the concurrent scanstore session lands: re-verify
      report citations vs new HEAD, re-shoot index render proof, re-audit
      go-cqrs-lite/cqrs-htmx verdicts (their event-store work may flip the
      premise).

## Harvested from the 2026-11-10 status report (docs-gates routing)

- [x] `AddRowChecked` for the 5-column invariant (S) — done (render.Table +
      runs ledger rows propagate add errors).
- [x] `RenderOptions.Title` for markdown/html output (S) — done (markdown
      pinned by test; html not registered in this binary).
- [x] `WithExample`/`WithLong` on the scan command (S) — done (plan v2 T4).
- [x] Add a "verified against commit X" line to every audit report header
      (point-in-time citation pinning) — done both halves: consumer-commit
      pins in all 22 audits (47d74ba) and scripts/audit-freshness.sh
      comparing each pinned version against go.mod (exit 1 on drift;
      currently 2 pinned audits fresh, 20 prospective SKIPs).
- [x] vendorHash in flake.nix is stale vs go.mod (nix-checker warning from
      the concurrent session's dep additions) — done: nix-hash-fix repaired
      it, hermetic nix build + buildflow gate green (2026-11-10).
- [x] README lacks an installation section (go-structure-linter info) —
      done: Install section (go install @latest, nix flake, UA note, tap
      planned post-tag) + honest json-vs-jsonl claims + journal quickstart
      incl. show/runs/--resume (2026-11-10).
- [x] Harvest status-report section (f) items into TODO_LIST.md
      (docs-health HARVEST) — done in this commit.
- [x] Update STACK.md when opportunities 1-2 (format widening, atomic
      write) are implemented — done: crash-safe-writes row (T3) + format
      enum row (T2); re-audit at plan v3 E21.
- [x] AGENTS.md: add the "reports are point-in-time @ commit" caveat next
      to the audit section — done: added above the Library audit section
      (2026-11-10).
- [x] `go mod tidy` sweep after the other session's feature stabilizes —
      done 2026-11-13: `go mod tidy` clean (zero go.mod/go.sum diff).

## Scanstore session follow-ups (docs/status/2026-11-10_01-12_*)

- [x] Error taxonomy in `classify.Counts`: break Errors into
      nxdomain/timeout/servfail — done: classify.ErrorKind end to end
      (counts breakdown, JSON error_kind, journal ErrKind, stderr summary),
      commit 49b87fc.
- [x] Sink retry/backoff instead of drop-after-first-error in scanstore
      recording — done: Sink retries each observation 3× with backoff,
      counts and reports dropped observations on Close.
- [x] cmdguard extras: WithCLICommit in nix build; WithExample/WithLong on
      scan — done (plan v2 T4, commit 6fc85ab).
- [x] RegisterUnknownRenderer(FormatJSON, dto) to dissolve the cli.go JSON
      special-case — done: render's init installs DTO renderers for json +
      jsonl over serialization's generic ones; renderResults dispatches via
      output.RenderUnknown (E26).
- [x] Lint-cleanup pass: pre-existing golangci warnings (err113,
      wrapcheck, varnamelen, tagalign, cyclop) to zero — done: `golangci-lint
      run` = 0 issues (2026-11-13); the CI lint job holds the line. plan
      v3 E24.
- [x] docs/status index (README) so status reports are discoverable —
      done: docs/status/README.md table (newest first) + cross-report
      truth notes (E1 negative-cache finding, flaky-test closure).
- [x] Version-freshness guard: CI/docs step flagging reports whose pinned
      lib released a newer tag — done: scripts/lib-freshness.sh in the CI
      lint job (audits every audit-pinned version vs latest upstream tag via
      `go list -m -versions`; live-verified: cmdguard v5.1.0, go-output
      v0.38.7 fresh).
- [x] TimeoutMiddleware whole-run budget for huge inputs — done:
      `--max-runtime` flag (applyRunBudget, plan v3 E31): aborts the run at
      the budget, partial results still render, journal, and count;
      interrupted-unscanned summary line reports the gap.
- [ ] Consider output.FormatsForShape to auto-generate the flag values
      enum from registered formats — plan v3 E27.

## Pareto plan execution (docs/planning/2026-11-10_00-50_*)

- [x] T4: CLI hardening bundle (validate tag, ExitError(2), WithEnvPrefix,
      RecoveryMiddleware) — done, commit 6fc85ab.
- [x] T5: real-world validation run before the tag — done: G3 PASS
      (report 07-46: 3645-domain .ch run, journal round-trip
      byte-identical, 12 Quad9 dig cross-checks, tail recovery sweep +17
      verdicts). NOTE (10:40 correction): the 07-46 "~90% of nxdomain
      bucket genuinely dead" claim is RETRACTED — it measured stale
      negative-cache state, not liveness (E1 replication sampled 11/12
      alive); positive verdicts unaffected.
- [x] T11: go-output polish (`--color`, SetFooter, Title, AddRowChecked)
      — done (2026-11-10: colors on scan+replay, footer table/markdown only,
      runs ledger title+footer, tests pinning all three; exit-2 for bad
      flag values via the cmdguard sentinel mapping).
- [x] T13: nom live progress during long scans (TTY-only) — done:
      go-output/nom InlineRenderer activity tree (✓/✗ per domain, 250ms
      redraw), gated on TTY stdout and never on jsonl; pipes byte-identical;
      scan.Options.OnDomainStart hook (nil-safe) drives the running set.
- [x] T20: `-o` implementation — resolved-as-decided-against (T1 kept
      `--format`; STACK.md decision log).
- [x] T22: registry-derived format enum, drift-proof — done (2026-11-10):
      render.SupportedFormats() derives from the go-output registries;
      TestFormatEnumsMatchRegistry pins all three --format tags to the
      contract and fails on a dropped blank import (E27).
- [x] T25: error-family exit codes (75 fetch / 65 corrupt) in
      internal/ranges — done: ranges.Error carries the kind and satisfies
      cmdguard's ExitCoder structurally; doctor names the family (E30).
- [x] T27: ROADMAP parking of deferred tail items — done: ROADMAP.md
      created (direction + parked ideas incl. CAA watch, compaction,
      lead scoring, DNSSEC-aware NXDOMAIN).
- [x] Mermaid compile-check of the plan graph; fix if broken — done: mmdc
      12.0.0 compiles the graph clean to SVG (2026-11-10).

## Harvested from the 07-46 T5-validated report (docs-gates freshness)

- [x] Push ALL pending commits to origin + github before tagging (g1)
      — done at release time and maintained since.
- [x] Watch the first real CI run on the pushed commits; fix findings
      — done: soak made opt-in (404dfed), gofmt drift fixed, CI green
      since; ci.yml now runs -timeout 20m and a nightly Soak workflow
      covers GDD_SOAK=1 (first dispatch run verified green).
- [x] Pull the .ch zone via the public SWITCH TSIG keys and build the .ch
      census input (docs/zone-data/switch-ch.md has the exact AXFR
      command; once/24h limit) — done 2026-11-13: AXFR pulled (ch.txt
      1.44 GB, TSIG key alongside) and reduced to 2,660,632 unique
      registrable apexes (zone-ch.txt, 43 MB, sha256 in MANIFEST.sha256;
      validation: 0 non-apex lines, 0 uppercase). Scanning the input
      waits for the Tranco waves to free the single-flight. Pre-filter
      superseded by the existence oracle.
- [x] crt.sh `.de` extractor rate-limit-aware scheduling — done:
      scripts/crtsh-daily.sh wrapper (24 h per-TLD guard is the contract,
      KEEP_COUNT rotation, plausibility floor: .de ≥ 100k rows else
      artifact rejected) + systemd user timer (04:37 + 20m jitter,
      Persistent). Wrong-root bug fixed 2026-11-13 (AGENTS.md gotcha);
      first legit verification = the next 04:37 timer run landing in-repo.
- [x] Re-verify the nxdomain bucket via a FRESH-cache resolver (unbound,
      census E7) — CLOSED by root cause 2026-11-13: not cache at all —
      scanner NXDOMAIN/NODATA conflation (see the census-lane block near the
      top of this file for the full chain: oracle fix, gdd-20261113,
      tranco1m-a-nxfix supersession).
- [ ] Fuzz the input parser (CSV quoting/encoding edges) — plan v3 E33.
- [x] IPv6 resolver-address handling check (`--resolver` only IPv4-tested)
      — done: TestNewWithAddrQueriesIPv6Server dials a real `[::1]` echo
      server (looping responder, atomic diagnostics); `[::1]:53`
      live-resolver gotcha in AGENTS.md; wave preflight splits bracketed
      IPv6 host:port correctly.
- [ ] TXT truncation investigation (cloudflare "cannot unmarshal" class).
- [x] vendorHash.nix extraction — resolved-as-deliberate-non-fix:
      buildflow's nix-hash-fix already owns hash updates; a separate
      vendorHash.nix adds indirection with zero behavior gain (decided
      2026-11-10).
- [ ] GPG signing policy decision for agent commits (toolship §f44) —
      owner decision.

## Harvested from the 06-58 scale-tier report

- [x] Redo the old-vs-new streaming memory A/B (PID-safe; the first
      attempt was killed by a pkill pattern match) and record the table
      (plan v3 E6) — done 2026-11-10: VmHWM A/B in
      reports/2026-11-10-e6-memory-ab/README.md; current jsonl flat
      (32 136 → 32 064 KB at 3 645 → 50 000), buffered json linear
      (28 080 → 41 624 KB, ~290 B/domain → ~700 MiB at 2.5M scale).
      Worktree removed, artifacts archived from /tmp.
- [x] ErrNoRuns error naming (names the store path at both cli call
      sites) — done; eyeball the describe battery output now that
      --resume/--wave exist (plan v3 E31) — still open.

## Harvested from the 10-11 v3-spine report

- [x] Fix the flaky cancel test "accordingly (deterministic cancel
      trigger first suspect)" — done: both Ctrl-C tests cancel from
      hangRes.LookupTXT when slow.com's lookup begins (strictly after
      run-start); wall-clock AfterFunc removed; full -race gates green
      twice consecutively (2026-11-10).

## Harvested from the 13-27 v0.1.0-release report (both sessions)

- [x] Scratch-dir `go get github.com/larsartmann/google-domain-detector@v0.1.0`
      pull-check once DNS recovers (proxy `.info` already verified) —
      done: `go get` + `go run ...@v0.1.0 version` compile and run from
      the proxy copy.
- [x] pkg.go.dev propagation spot-check (fetch the pkg page) once DNS
      is stable — done: page live at v0.1.0 with full README, MIT
      license, directories rendered.
- [x] Consumer propagation sweep for v0.1.0 (go-ecosystem-upgrade skill;
      only relevant if external consumers exist — likely none yet) — done
      2026-11-13: GitHub code search for the module path = 32 hits, ALL
      inside this repo; zero external Go consumers. The Kith CRM consumes
      CLI outputs (CSV/JSON), not the module (pointer in AGENTS.md).
- [x] `git worktree remove /tmp/gdd-pret7` after the E6 memory A/B
      finishes (sibling session's pre-T7 worktree) — done 2026-11-10.
- [x] Unbound tuning experiment (threads/outgoing-range) before the E9
      throughput bench — done and SUPERSEDED: tuning tested, but cold
      recursion stalls ≥4 workers from upstream-flow pressure; forwarder
      mode (Quad9) is the fix. scripts/unbound.conf + docs/census.md.
- [x] Nightly soak CI job decision — decided + implemented: scheduled
      Soak workflow (03:17 UTC, GDD_SOAK=1, 45m budget) + manual dispatch;
      push CI stays fast.
- [x] Add `gh run list` to the session-start checklist so CI status on
      recent pushes is seen immediately (the red-on-tag lesson) —
      adopted in project AGENTS.md (Gotchas).
- [x] E33 cli package coverage 69% → 90% (§f29 of this report; also
      06-09 §f28) — merged into the E33 row above; done at 90.7%.

## Harvested from the 01-12 and 06-09 reports (docs-gates audit-all)

- [x] Streaming/chunked input loader (30M-domain scale) — done: chunked
      domainFeeder + input.Streamer (plan v2 T8); buffered formats still
      materialize their report (bulk = jsonl).
- [x] WithOutputFormat adoption + JSON error envelopes — resolved-as-
      decided-against with `-o` (T1; STACK.md decision log); error
      envelopes may return post-tag if a need appears.
- [ ] Enrichment join: imprint/contact crawl for the top-N matches
      (legal care: robots/ToS) or company-master-data cross-ref —
      post-census idea (06-09 §f12).
- [ ] Dedup by registrant/organization for portfolio-owned domain
      clusters — needs zone/WHOIS data first (06-09 §f13).
- [ ] M365/Exchange fingerprints (outlook MX/SPF includes) — one scanner,
      full email-stack picture; detection-rule addition, classify-only
      (06-09 §f14).
- [x] SWITCH terms (.ch) — done: verified terms + public TSIG access in
      docs/zone-data/switch-ch.md (report 07-46).
- [x] golangci-lint + govulncheck CI steps — done: ci.yml `lint` job
      (golangci-lint-action v9.3.0 with v2.15.0 + govulncheck@v1.9.0);
      first run green 2026-11-13. plan v3 E25 (06-09 §f29).
- [x] cli package coverage 69% → 90+ — plan v3 E33 — merged into the
      E33 coverage row under the 13-27 harvest section (single owner).
- [ ] asciinema/GIF for the README — with T14/README work (plan v3 E20).
- [x] Feed-freshness/UA integration note once the README install section
      lands — done: the UA note lives in the Install section since the
      README overhaul (2026-11-10), kept current through the 2026-11-13
      polish (tap install added, enrich + --max-runtime documented).
      plan v3 E20.
