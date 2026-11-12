# Status 2026-11-11 22:05 — census hardening while wave A runs

Continuation of the census lane (15-27 handoff). Owner §g questions
still unanswered → operating on documented defaults (w4 accepted, no
foreign-lane harvesting, local tarball per wave).

## Live state

- **Wave A pass 1:** 55,334/90,834 scanned (60.9%), unknown 923 =
  **1.67%** (steady under the 2% D3 bar; was 1.75% at 19%, 1.70% at
  54%). Unbound healthy. Rate ~94/min → ETA ~04:15 CET. Wrapper
  (pids 91698/92164, old script inode) self-manages gate + cleanup.
- CI green on tip; soak cron's second scheduled fire green (run
  42952246214, 14m39s).

## Done this stretch

1. **Mid-pass unknown-rate WARNING** (5cf2231): census-wave.sh poller
   thresholds the running unknown rate against the D3 bar (env
   `GDD_WARN_PCT`, default 2, `0` disables); boundary-tested, then
   swapped **inode-safely** (`install`, new inode) so the running wave
   A wrapper kept its loaded bytes — waves B/C get the WARNING.
   Documented in docs/census.md.
2. **Binary provenance stamped** (§e2): the census binary's embedded
   VCS stamp (`go version -m`) is authoritative — `e1f5160` + dirty
   tree, vcs.time 2026-11-10T18:12:02Z, go1.27.2/jsonv2. sha256 +
   verification recipe in `reports/census/unbound/PROVENANCE.md` +
   NOTES.md. Byte-reproduction was attempted and is impossible by
   design (build-dir paths embedded); recorded as such. Decision:
   waves B/C reuse THIS binary — one instrument for the census.
3. **audit-freshness.sh wired into CI** (068b37e) — new "Audit
   freshness" step green on its first run. Its first live catch was
   real: **cmdguard had moved to v5.1.0 in go.mod** (daemon-swept
   16:30, sibling/go-ecosystem lane) while the audit pinned v5.0.0.
   Re-verified the audit against the tag: Go-source delta over v5.0.0
   is +101 non-test lines (the two previewed opt-ins, mutex Builder,
   doctor doc-fixes) — no cited API changed → pin bumped to v5.1.0
   with the verification note. No sibling files touched.
4. **Census journal snapshots** (04efbfd + daemon b7b63a6): new
   `scripts/census-snapshot.sh` — sqlite online backup (WAL-consistent
   under load) → integrity_check → tar.gz + sha256 sidecar.
   Live-verified against the running journal (205 MB → 33 MB, restore
   - row-count proof: meta_stream_log 72,510 rows). The wave runner
     now auto-snapshots into `reports/census/snapshots/` on completion
     (both exit paths, stub-tested). Wave A (old inode) gets a **manual**
     snapshot at completion — see protocol below.
5. **Outage-window CI runs eyeballed** (§f12): b22b6fc / c6dca0c /
   3b3ae41 / 92432b0 — all steps success, nothing skipped or masked.
6. Soak cron verification (§f11) — see live state.

## Wave A completion protocol (for the ~04:15 session)

1. `tail reports/census/waves/tranco1m-a/monitor.log` — expect
   "pass 1 finished", gate verdict, and (if FAIL) cleanup rounds.
2. If the old wrapper's gate ran PASS: manual snapshot —
   `./scripts/census-snapshot.sh reports/census/census.db reports/census/snapshots tranco1m-a-complete`
   (the old inode predates auto-snapshot).
3. Single-flight check (`pgrep -f 'gdd scan'` — must be empty), then
   launch wave B (wrapper mkdirs its own out-dir):
   `setsid nohup ./scripts/census-wave.sh reports/census/unbound/gdd reports/census/census.db tranco1m-b reports/census/waves/tranco1m-b.txt reports/census/waves/tranco1m-b </dev/null >/dev/null 2>&1 &`
4. Strike the T11 wave-A half in TODO_LIST.md (freshness --pre-flight
   first, distinctive token: T11).

## Still open / owner-blocked

- §g1 w4-vs-VPS, §g2 cross-lane harvesting, §g3 snapshot destination
  beyond local tarballs — unchanged, defaults in force.
- docs-gates: audit-all + cadence still DELIBERATE reds (18 unarchived
  reports now — a sibling added 17-43 cqrs-event-sourcing-audit-status;
  archive ritual awaits owner).
- flake vendorHash vs the cmdguard v5.1.0 bump: TODO_LIST row was
  already struck [x] (line 154) — believed handled by the bumping
  lane; flagged here so the next nix build surprises nobody.
- E17 census report: after wave C; excluded pilot run ID to pin:
  `01M74VNNCJS6` (now greppable here too).

**Lane discipline kept:** internal/cli + internal/scanstore untouched;
the only sibling-adjacent file changed was docs/review/libs/cmdguard.md
(own E31.4 audit, re-pinned with verification evidence).
