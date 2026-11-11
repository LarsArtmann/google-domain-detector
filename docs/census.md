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
- Ruleset: waves run under `classify.CurrentRuleset` (stamped into every
  event; provenance test pins it). Any detection-rule change MUST bump
  the ruleset before the next wave.

### Wave ledger

| Wave       | Ranks             | List  | Manifest sha256 (first 12) | Status  |
| ---------- | ----------------- | ----- | -------------------------- | ------- |
| tranco1m-a | 1–100 000         | 947N2 | b4450f660567               | pending |
| tranco1m-b | 100 001–500 000   | 947N2 | b6d5b3821c90               | pending |
| tranco1m-c | 500 001–1 000 000 | 947N2 | b0317d71873a               | pending |

## Worker-pressure findings (E7/E9 ladder, measured 2026-11-10)

Two resolver modes were measured, cold cache, 10s timeout, mixed-TLD
Tranco slices (worst-case single-TLD .ch noted where it differs):

| Mode           | Workers | dom/s   | unknown% | Gate     |
| -------------- | ------- | ------- | -------- | -------- |
| full recursion | 2       | 2.9     | 3.0%     | FAIL     |
| full recursion | 4       | 1.2     | 19.5–24% | FAIL     |
| full recursion | 8       | 0.7     | 20–22%   | FAIL     |
| **forwarder**  | 4       | 3.7     | 0.5%     | PASS     |
| **forwarder**  | **8**   | **4.1** | **1.5%** | **PASS** |
| forwarder      | 16      | 1.9     | 65.5%    | FAIL     |

Full recursion from this host collapses under scan bursts: cold
recursions take 10–20s+ at ≥4 workers (every victim resolves instantly
in isolation; unbound's front door stayed responsive throughout), which
matches upstream-flow pressure on the local NAT path, not resolver CPU.
Forwarding to Quad9 (scripts/unbound.conf) moves recursion onto
infrastructure built for it; census qps stays far under bulk-throttle
thresholds. Warm cache at any worker count: clean and fast.

**Wave parameters: workers 8, timeout 10s, resolver 127.0.0.1:5353
(forwarder mode).** ~4.1 domains/s ⇒ wave A ≈ 7 h, full 1M ≈ 2.9 days —
resumable by design (`--resume latest`); cleanup loops re-run unknowns
at low workers until < 0.5%. Sustained-throttle contingency: the gate
runs after every pass; a throttle window surfaces as elevated timeouts
and is absorbed by the cleanup loop (optionally add 1.1.1.1 as a third
forwarder — documented lever, deliberately homogeneous Quad9 for now).

Single-TLD scans (.ch zone pulls): the same ladder's recursion-mode w2
row is the relevant bound — workers ≤ 2 plus cleanup loops, always.
