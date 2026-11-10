# TODO List

Short- and mid-term improvement tasks for google-domain-detector. Status
markers: `[ ]` open, `[~]` in progress, `[x]` done.

## Census: Tranco top-1M (plan: docs/planning/2026-11-10_06-18_tranco-1m-census-event-sourced.md)

Ownership (agreed with the concurrent v2-journal plan,
docs/planning/2026-11-10_01-18_*): that plan stays CANONICAL for
scanstore/CLI-internal work (its T-series, plans T3/T4/T9/T10 already
landed); this census plan owns everything downstream of the journal:
waves, manifests, unbound, pilot gates, reports, deltas.

- [ ] T02 Evidence-schema version (detection ruleset ID) + wave ID stamped
      into events BEFORE any census wave.
- [ ] T05 Unbound spike: user-scoped nix shell, 127.0.0.1:5353, probe via
      `--resolver` — removes the public-resolver throttle wall.
- [ ] T07 Pilot harness: slice-Nk + error-rate gate + pre-flight doc.
- [ ] T08 Throughput benchmark: worker sweep on a 10k slice → wave params.
- [ ] T09 Recover the 7,204 legacy unknowns (.de/.ch/.uk) via unbound +
      `--resume`.
- [ ] T10 Tranco full-list import + wave manifests (frozen rank bands).
- [ ] T11–T13 Waves A (1–100k), B (100–500k), C (500k–1M) with pilot gates.
- [ ] T14 Unknown-cleanup loop per wave (<0.5% unknown).
- [ ] T15 Census report (journal-derived) archived under reports/.
- [ ] T16 Delta query: verdict flips between runs (the sales signal).

## Detection coverage

- [x] Batch retry (once) for transient resolver failures — done:
      retryResolver in internal/scan (transient only, NXDOMAIN never
      retried), commit 49b87fc.
- [ ] Own recursive resolver guidance for whole-zone scale (unbound/knot) —
      public resolvers throttle bulk scanners; results stay ~11 % unknown
      otherwise (see reports/2026-11-09-tranco-de-ch-uk). = census T05.
- [x] `--jsonl` streaming output for very large batches (render results as
      they complete instead of after the whole run) — done: render
      JSONLStream + scan/replay streaming paths.
- [x] Resume/checkpointing for multi-hour runs — done differently: the
      event journal IS the checkpoint (`--resume ID|latest` skips observed
      domains, links runs via scan.started), commit 3674285.
- [ ] Consider CAA/other record classes only if a real fingerprint emerges —
      do not add speculative probes.
- [ ] Optional CSV enrichment mode: emit the input CSV with appended
      workspace/gcp columns, preserving all original columns. = census T18.

## Whole-TLD coverage (the "all active .de/.ch/.uk" goal)

Tranco top-1M covers the traffic-active subset (~63k of the ~30M registered
domains across the three TLDs). Full zone coverage needs the registry data,
which is paperwork only the account holder can sign:

- [x] .de (~17.5M registered): DENIC RDS — RESOLVED-AS-IMPOSSIBLE: DENIC
      refuses ALL third-party zone access (no program, no fee, no
      contact; FAQ verbatim in docs/zone-data/denic-de.md). .de coverage
      = Tranco + crt.sh fallback only.
- [ ] .uk (~10.5M): Nominet Data Access Agreement (DAA), free but requires
      application + acceptance.
- [x] .ch (~2.4M): SWITCH zone file — ACTIONABLE NOW: public TSIG keys on
      zonedata.switch.ch, `dig -k ch_zonedata.key @zonedata.switch.ch …
      AXFR ch.` (once/24h; exact command in docs/zone-data/switch-ch.md);
      no owner paperwork. Follow-up task below (harvested 07-46).
- [ ] Pre-filter before scanning at that scale: a cheap single-A-record
      existence pass (or NXDOMAIN pre-sweep) would drop the parked share and
      cut scan volume several-fold.
- [ ] Memory: buffered formats (table/json/csv) still materialize the full
      report — fine at 100k domains, not at 30M. Bulk guidance: use
      `--format jsonl` (streams per result; input already chunks at 4096;
      50k-domain smoke = 40 MiB RSS flat). The original "input loads fully
      into RAM" concern is landed-fixed via the streaming feeder.

## Operational

- [ ] Release v0.1.0 tag once the first real-world lead list ran clean
      (go-release skill owns the process).
- [ ] Homebrew tap entry (homebrew-tap repo) after the tag.
- [ ] nix run .#release wiring check (go-nix-helpers release app).

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
- [ ] cmdguard-lint as a CI step (expected zero findings).
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
- [ ] Add a "verified against commit X" line to every audit report header
      (point-in-time citation pinning).
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
- [ ] `go mod tidy` sweep after the other session's feature stabilizes.

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
- [ ] Lint-cleanup pass: pre-existing golangci warnings (err113,
      wrapcheck, varnamelen, tagalign, cyclop) to zero — plan v3 E24
      (gate G5: after sibling cli/render lane lands).
- [x] docs/status index (README) so status reports are discoverable —
      done: docs/status/README.md table (newest first) + cross-report
      truth notes (E1 negative-cache finding, flaky-test closure).
- [ ] Version-freshness guard: CI/docs step flagging reports whose pinned
      lib released a newer tag.
- [ ] TimeoutMiddleware whole-run budget for huge inputs — plan v3 E31.
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
- [ ] T25: error-family exit codes (75 fetch / 65 corrupt) in
      internal/ranges — plan v3 E30.
- [ ] T27: ROADMAP parking of deferred tail items — plan v3 E31.
- [x] Mermaid compile-check of the plan graph; fix if broken — done: mmdc
      12.0.0 compiles the graph clean to SVG (2026-11-10).

## Harvested from the 07-46 T5-validated report (docs-gates freshness)

- [ ] Push ALL pending commits to origin + github before tagging (g1).
- [ ] Watch the first real CI run on the pushed commits; fix findings.
- [ ] Pull the .ch zone via the public SWITCH TSIG keys and build the .ch
      census input (docs/zone-data/switch-ch.md has the exact AXFR
      command; once/24h limit) — feeds the pre-filter (plan v3 E28).
- [ ] Re-verify the nxdomain bucket via a FRESH-cache resolver (unbound,
      census E7) before quoting any coverage number — the E1 replication
      found stale negative-cache lies at ~92% of the sampled bucket; D3's
      tail bar is only assessable on unbound-backed runs. (plan v3 E7/E10
      feed this.)
- [ ] cli coverage 69% → 90% (plan v3 E33).
- [ ] Fuzz the input parser (CSV quoting/encoding edges) — plan v3 E33.
- [ ] IPv6 resolver-address handling check (`--resolver` only IPv4-tested)
      — plan v3 E31.
- [ ] TXT truncation investigation (cloudflare "cannot unmarshal" class).
- [ ] vendorHash.nix extraction — plan v3 E31.
- [ ] GPG signing policy decision for agent commits (toolship §f44) —
      owner decision.

## Harvested from the 06-58 scale-tier report

- [ ] Redo the old-vs-new streaming memory A/B (PID-safe; the first
      attempt was killed by a pkill pattern match) and record the table
      (plan v3 E6).
- [ ] ErrNoRuns error should name the store path; eyeball the describe
      battery output now that --resume/--wave exist (plan v3 E31).

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
- [ ] golangci-lint + govulncheck CI steps — plan v3 E25 (06-09 §f29).
- [ ] cli package coverage 69% → 90+ — plan v3 E33.
- [ ] asciinema/GIF for the README — with T14/README work (plan v3 E20).
- [ ] Feed-freshness/UA integration note once the README install section
      lands (plan v3 E20).
