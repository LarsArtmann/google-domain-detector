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

- [ ] .de (~17.5M registered): DENIC RDS (Registered Domain Data Service)
      zone data — member/researcher agreement with DENIC.
- [ ] .uk (~10.5M): Nominet Data Access Agreement (DAA), free but requires
      application + acceptance.
- [ ] .ch (~2.4M): SWITCH zone file under their research/business terms.
- [ ] Pre-filter before scanning at that scale: a cheap single-A-record
      existence pass (or NXDOMAIN pre-sweep) would drop the parked share and
      cut scan volume several-fold.
- [ ] Memory: input currently loads fully into RAM (~30M domains ≈ 1.5 GB) —
      stream input in chunks before any whole-zone run. — UPDATE: input
      streams in 4096-domain chunks since the streaming feeder landed
      (internal/cli/domains.go); this row is about RENDERED OUTPUT of
      buffered formats only. Downgrade to jsonl-for-bulk guidance.

## Operational

- [ ] Release v0.1.0 tag once the first real-world lead list ran clean
      (go-release skill owns the process).
- [ ] Homebrew tap entry (homebrew-tap repo) after the tag.
- [ ] nix run .#release wiring check (go-nix-helpers release app).

## Library audit follow-ups (2026-11-10, plan: docs/planning/2026-11-10_00-50_*)

- [ ] Widen `--format` enum to +yaml,toml,jsonl (renderers already
      registered via blank imports — one flag-tag string).
- [ ] Adopt go-atomic-write `WriteWithPerm` in `ranges.writeCache` (never
      `WriteIfChanged`: it would skip the mtime refresh the 24 h TTL needs).
- [x] CLI hardening: `validate` tag on --workers, `ExitError(2)` for usage
      errors, `WithEnvPrefix("GDD_")`, RecoveryMiddleware — done (ScanFlags
      validate:"min=1,max=512", exitCodeUsage, env prefix + recovery wired
      in cliOptions).
- [ ] `--color auto|always|never` via go-output ColorMode; SetFooter summary
      row; AddRowChecked.
- [ ] `WithDoctorCheck` preflight (resolver + ranges feed).
- [ ] Decide `--format` vs cmdguard's `-o/--output` BEFORE the v0.1.0 tag
      (breaking rename is cheap now, never later).
- [ ] cmdguard-lint as a CI step (expected zero findings).
- [ ] Audit upkeep after the concurrent scanstore session lands: re-verify
      report citations vs new HEAD, re-shoot index render proof, re-audit
      go-cqrs-lite/cqrs-htmx verdicts (their event-store work may flip the
      premise).

## Harvested from the 2026-11-10 status report (docs-gates routing)

- [ ] `AddRowChecked` for the 5-column invariant (S).
- [ ] `RenderOptions.Title` for markdown/html output (S).
- [ ] `WithExample`/`WithLong` on the scan command (S).
- [ ] Add a "verified against commit X" line to every audit report header
      (point-in-time citation pinning).
- [x] vendorHash in flake.nix is stale vs go.mod (nix-checker warning from
      the concurrent session's dep additions) — done: nix-hash-fix repaired
      it, hermetic nix build + buildflow gate green (2026-11-10).
- [ ] README lacks an installation section (go-structure-linter info).
- [x] Harvest status-report section (f) items into TODO_LIST.md
      (docs-health HARVEST) — done in this commit.
- [ ] Update STACK.md when opportunities 1-2 (format widening, atomic
      write) are implemented.
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
- [ ] cmdguard extras: WithCLICommit in nix build; WithExample/WithLong on
      scan.
- [ ] RegisterUnknownRenderer(FormatJSON, dto) to dissolve the cli.go JSON
      special-case.
- [ ] Lint-cleanup pass: ~29 pre-existing golangci warnings (err113,
      wrapcheck, varnamelen, tagalign, cyclop) to zero.
- [ ] docs/status index (README) so status reports are discoverable.
- [ ] Version-freshness guard: CI/docs step flagging reports whose pinned
      lib released a newer tag.
- [ ] TimeoutMiddleware whole-run budget for huge inputs.
- [ ] Consider output.FormatsForShape to auto-generate the flag values
      enum from registered formats.

## Pareto plan execution (docs/planning/2026-11-10_00-50_*)

- [ ] T4: CLI hardening bundle (validate tag, ExitError(2), WithEnvPrefix,
      RecoveryMiddleware).
- [ ] T5: real-world validation run (lead list / Tranco slice) before the tag.
- [ ] T11: go-output polish (`--color`, SetFooter, Title, AddRowChecked).
- [ ] T13: nom live progress during long scans (TTY-only).
- [ ] T20: `-o` implementation if T1 says adopt.
- [ ] T22: registry-derived format enum, drift-proof (FormatsForShape).
- [ ] T25: error-family exit codes (75 fetch / 65 corrupt) in internal/ranges.
- [ ] T27: ROADMAP parking of deferred tail items.
- [x] Mermaid compile-check of the plan graph; fix if broken — done: mmdc
      12.0.0 compiles the graph clean to SVG (2026-11-10).
