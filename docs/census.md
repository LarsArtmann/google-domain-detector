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
0.5 = cleanup-loop target).

## Waves

- Frozen manifests under `reports/census/waves/` (`tranco1m-a/b/c.txt`,
  sha256 in `MANIFEST.sha256`). NEVER re-cut mid-census; a new Tranco
  freeze is a new wave with a new ID.
- Tranco list IDs and manifest hashes are recorded in the wave ledger
  below. Current freeze: list `947N2` (2026-11-10).
- Every wave run uses `--event-store reports/census/census.db --wave
  <id>` so the journal is the source of truth; interrupted waves resume
  via `--resume latest` (observed domains are skipped — never
  blind-rescan, census guard 3).
- Ruleset: waves run under `classify.CurrentRuleset` (stamped into every
  event; provenance test pins it). Any detection-rule change MUST bump
  the ruleset before the next wave.

### Wave ledger

| Wave        | Ranks          | List  | Manifest sha256 (first 12) | Status |
| ----------- | -------------- | ----- | -------------------------- | ------ |
| tranco1m-a  | 1–100 000      | 947N2 | b4450f660567               | pending |
| tranco1m-b  | 100 001–500 000| 947N2 | b6d5b3821c90               | pending |
| tranco1m-c  | 500 001–1 000 000 | 947N2 | b0317d71873a             | pending |

## Worker-pressure findings (E7/E9, measured 2026-11-10)

Measured on single-TLD (.ch) lists through local unbound — the worst
case, all queries hammer one TLD's nameservers:

- workers 2, cold: 3.0% unknown; warm: clean.
- workers 4, cold: 19.5% unknown (73/400 timeouts) — TLD nameserver
  rate limiting under burst; every victim resolves instantly in
  isolation afterwards.
- workers 8, cold: 20–22% unknown; warm w8: 1% unknown, 22 domains/s.

Consequences:

1. **Wave runs** (mixed-TLD Tranco): concurrency is bounded by TLD-NS
   politeness, not by local CPU. Use the worker count the E9 mixed-TLD
   ladder measured (see `reports/census/unbound/NOTES.md`), expect the
   first pass to leave unknowns, and let the cleanup loop
   (low-worker re-runs via `--resume`) collapse them — the gate applies
   to the post-cleanup state.
2. **Single-TLD scans** (.ch zone pulls etc.): workers ≤ 2 plus cleanup
   loops, always.
