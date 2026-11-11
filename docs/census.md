# Census runbook — Tranco top-1M

Operational doc for running census waves (plan:
`docs/planning/2026-11-10_06-18_tranco-1m-census-event-sourced.md`, E7–E18 of
unified plan v3). Bulk data lives under `reports/census/` (gitignored); this
doc and everything under `scripts/` are tracked.

## Resolver

Bulk scans NEVER use public resolvers directly — they throttle sustained
bulk clients (proven: two T5 runs poisoned; see
`docs/status/2026-11-10_06-58_*`). The census resolver is a user-scoped
recursive unbound on `127.0.0.1:5353`:

```bash
scripts/unbound.sh start     # also: stop | status | restart
```

- Canonical config: `scripts/unbound.conf` (16 threads, outgoing-range
  8192, serve-expired, prefetch). Tuned 2026-11-10 after the E7
  worker-pressure measurements (see `reports/census/unbound/NOTES.md`).
- Never runs as root, never becomes the system resolver (census guard 5).

## Error semantics (D3 tail, decided 2026-11-10)

- **NXDOMAIN is domain truth** — an authoritative "this name does not
  exist". Counted in coverage; never retried (owner default D1).
- **timeout / SERVFAIL / canceled / other are tool-environment states** —
  they say nothing about the domain. Excluded from coverage claims,
  retried by cleanup loops, and never scaled past (the pilot gate fails
  on them). A SERVFAIL from our own overloaded resolver is not a fact
  about google-adoption; counting it against coverage would read
  resolver capacity as domain state.
- **Refinement (E10, 2026-11-10): stable SERVFAIL is domain truth.** A
  SERVFAIL that persists across ≥2 cleanup rounds through a verified
  healthy forwarder is a domain-side broken delegation (dead or
  misconfigured authoritative servers), not tool state. It counts as
  _explained_ in coverage — but never silently: confirm resolver health
  first, then pass `--servfail-stable` to `census-gate.sh` so the
  reclassification is explicit in the gate output. E10 evidence: 567
  SERVFAILs stable across two rounds (r1 heavy-load, r2 healthy
  forwarder w4), archived under `reports/census/unbound/raw/`.

## Pilot gate (G6)

Before every 10x scale jump, run the gate:

```bash
scripts/census-slice.sh reports/census/waves/tranco1m-a.txt 5000 /tmp/pilot.txt
gdd scan --no-ranges --workers <W> --timeout <T> --resolver 127.0.0.1:5353 \
    --event-store reports/census/census.db --wave <wave-id> \
    --input /tmp/pilot.txt --format jsonl > /tmp/pilot.jsonl 2> /tmp/pilot.stderr
scripts/census-gate.sh /tmp/pilot.stderr          # PASS required (< 2% unknown)
scripts/census-gate.sh /tmp/pilot.jsonl 0.5       # cleanup-loop bar
```

`census-gate.sh` accepts the stderr run summary or jsonl output; the
second argument overrides the unknown budget (2 = D3 bar default,
0.5 = cleanup-loop target). `--servfail-stable` reclassifies SERVFAIL
as explained (broken delegation) — only after the ≥2-round stability
check above; the flag exists so the reclassification is a visible
operator decision, not an accounting default.

## Waves

- Frozen manifests under `reports/census/waves/` (`tranco1m-a/b/c.txt`,
  sha256 in `MANIFEST.sha256`). NEVER re-cut mid-census; a new Tranco
  freeze is a new wave with a new ID.
- Non-Tranco zone inputs: `.ch` zone pulls via the public SWITCH TSIG
  keys (docs/zone-data/switch-ch.md, once/24h) and certificate
  transparency via `scripts/crtsh-extract.sh <tld>` (the only `.de`
  fallback — DENIC refuses all third-party zone access; crt.sh is
  rate-limited, at most once a day per TLD; `--test` runs its fixture
  self-test).
- Tranco list IDs and manifest hashes are recorded in the wave ledger
  below. Current freeze: list `947N2` (2026-11-10).
- Every wave run uses `--event-store reports/census/census.db --wave
  <id>` so the journal is the source of truth; interrupted waves resume
  via `--resume latest` (observed domains are skipped — never
  blind-rescan, census guard 3).
- Cleanup loop (E16): after a wave pass, drive unknowns under the bar:
  `scripts/census-cleanup.sh <gdd-binary> reports/census/census.db
  <wave-id> <wave-jsonl> [max-rounds=3]` — re-scans only unknown-class
  domains at falling workers (round 1: 4, later: 2) as journal runs
  `<wave>-c<N>`. The bar (0.5%) applies to the ORIGINAL wave size, not
  the shrinking round size: a round's own unknown-rate is meaningless
  (its input is 100% prior errorers). Afterwards, assess residual
  SERVFAILs for stability with `census-gate.sh --servfail-stable`
  (D3 refinement above).
- Wave runner (`scripts/census-wave.sh`) wraps one wave: single-flight
  check, pass 1 via `--resume latest`, a 30-min progress poller, the
  gate, and on FAIL the cleanup loop — all appended to
  `<out-dir>/monitor.log` so a detached wave survives its session. The
  poller logs a WARNING when the running unknown rate exceeds the D3
  bar (env `GDD_WARN_PCT`, default `2` = percent; `0` disables). On
  completion (either path) it snapshots the journal (below).
- Journal snapshots (§g3 default, local tarball per completed wave):
  `scripts/census-snapshot.sh <store> <snapshots-dir> [label]` —
  sqlite online backup (WAL-consistent, safe to run mid-scan) then
  tar.gz + sha256 sidecar. The wave runner calls it automatically into
  `reports/census/snapshots/` with label `<wave>-complete-<ts>`;
  wave A (launched before this existed) gets a manual snapshot at
  completion.
- Ruleset: waves run under `classify.CurrentRuleset` (stamped into every
  event; provenance test pins it). Any detection-rule change MUST bump
  the ruleset before the next wave.

### Wave ledger

| Wave       | Ranks             | List  | Manifest sha256 (first 12) | Status  |
| ---------- | ----------------- | ----- | -------------------------- | ------- |
| tranco1m-a | 1–100 000         | 947N2 | b4450f660567               | in progress (w4 soaks 1–3 + pilot collision; main pass running) |
| tranco1m-b | 100 001–500 000   | 947N2 | b6d5b3821c90               | pending |
| tranco1m-c | 500 001–1 000 000 | 947N2 | b0317d71873a               | pending |

## Worker-pressure findings (E7/E9 ladder + 2026-11-11 soaks)

Two resolver modes were measured, cold cache, 10s timeout, mixed-TLD
Tranco slices (worst-case single-TLD .ch noted where it differs):

| Mode           | Workers | dom/s   | unknown% | Gate     |
| -------------- | ------- | ------- | -------- | -------- |
| full recursion | 2       | 2.9     | 3.0%     | FAIL     |
| full recursion | 4       | 1.2     | 19.5–24% | FAIL     |
| full recursion | 8       | 0.7     | 20–22%   | FAIL     |
| forwarder      | 4       | 3.7     | 0.5%     | PASS     |
| forwarder      | 8       | 4.1     | 1.5%     | PASS     |
| forwarder      | 16      | 1.9     | 65.5%    | FAIL     |

The table above is ~50-second windows. **Sustained 30-minute soaks
(2026-11-10/11, artifacts under
`reports/census/waves/tranco1m-a/soak{,2,3}.*`) tell a different
story:**

| Soak | Conf               | Workers | dom/s | unknown% |
| ---- | ------------------ | ------- | ----- | -------- |
| 1    | Quad9 only         | 8       | 1.75  | 26.0%    |
| 2    | Quad9 + Cloudflare | 8       | 1.72  | 21.1%    |
| 3    | Quad9 + Cloudflare | 4       | 1.52  | 5.35%*   |

\* soak 3 shared the resolver with a concurrently-running pilot scan
for part of its window (single-flight law below) — 5.35% is an upper
bound. Unknown scales super-linearly with workers under sustained
load; a second forwarder helps only marginally at w8.

**Wave parameters (decided 2026-11-11): workers 4, timeout 10s,
resolver 127.0.0.1:5353 (Quad9 + Cloudflare forwarder).** ~1.5 dom/s ⇒
wave A ≈ 16–18 h, full 1M ≈ 7–8 days — resumable by design
(`--resume latest`). Two-pass protocol: the first pass LEAVES
unknowns, the cleanup loop (E16) drives them under 0.5%, and the D3
gate runs POST-CLEANUP. Timeouts burn their full per-domain budget, so
a timeout-heavy first pass is also the slow one — at w4 the expected
first-pass residue is ≤5%: cleanup-territory, not pathology.

**Single-flight law: ONE census scan at a time.** Concurrent scans
share 127.0.0.1:5353 and poison each other's measurements — proven
2026-11-11 09:33 (a pilot and a soak ran concurrently; both showed
inflated unknowns). Before launching: `pgrep -f 'gdd scan'` must be
empty, and the wave lane owns long runs. Bulk scans also NEVER use the
system resolver directly — a 2026-11-11 pilot accidentally did (its
observations went through 127.0.0.1:53 and are excluded from census
accounting as protocol-contaminated).

Single-TLD scans (.ch zone pulls): the same ladder's recursion-mode w2
row is the relevant bound — workers ≤ 2 plus cleanup loops, always.
