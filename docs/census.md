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

### Two measured UDP defects and the TCP-upstream variant (2026-11-13)

The UDP forwarder path has two independent, measured failure modes; both
are reasons the census default moves to the TCP-upstream variant:

1. **NXDOMAIN fabrication under census load.** During the 2026-11-10
   pilot and the 2026-11-13 patch cleanup round 4, live domains got
   NXDOMAIN-classified verdicts through the UDP forwarder (12/12 sampled
   false, verified by direct `dig @9.9.9.9` — see
   `docs/status/2026-11-13_01-04_census-plan-execution-and-resolver-forensics.md`).
   Worker-count independent (seen at w8 and w2). Smoking gun in the
   unbound log: `cannot increase max open fds from 65536 to 131566 —
   continuing with less udp ports: 4065` — the outgoing UDP port pool
   (unbound's UPSTREAM query pool) is starved by the user-scoped fd
   limit, and under starvation the forwarder path returns wrong
   authoritative-looking negatives. False NXDOMAINs are the worst census
   error class: NXDOMAIN counts as domain truth, so they would
   permanently pollute the census.
2. **Host-level outbound UDP:53 outage (2026-11-12 23:24–23:45).**
   Every fresh UDP recursion died (`dig @9.9.9.9` = no servers
   reachable) while TCP:53 and ICMP kept working; a wave launched into
   that window fabricated ~3 450 garbage observations. The negative
   entries OUTLIVE the distress (see AGENTS.md "distressed resolver"
   gotcha).

**Mitigation: `scripts/unbound-tcp.conf` — TCP-upstream forwarding on
`127.0.0.1:5354`** (`tcp-upstream: yes`: the unbound → upstream hop is
TCP end to end, connection-scoped, no UDP-port-pool surface; the
scanner → unbound hop stays loopback UDP — Go's resolver can't do
DNS-over-TCP via its Dial callback without breaking framing, so
scanner-side TCP is not an option). Cost: wall-clock — per-query
upstream connections are slower than UDP; a fresh 5k pilot through this
config measures the throughput and is the launch gate for any wave on
it. Usage (per-instance lifecycle, never bare nohup):

```bash
GDD_UNBOUND_CONF=scripts/unbound-tcp.conf scripts/unbound.sh start   # also: status | stop | restart
# scans then target it with --resolver 127.0.0.1:5354
```

### Pre-flight law (2026-11-13)

`census-wave.sh` refuses to start (exit 5) unless BOTH two upstream
resolvers and the configured forwarder answer a live probe. A forwarder
answering from cache proves nothing about upstream health — during the
UDP outage `unbound.sh status` was green while every fresh recursion
died. Waves on the TCP variant set `GDD_PREFLIGHT_TRANSPORT=tcp` so the
upstream probes run over TCP too (immune to a UDP-only outage);
`GDD_NO_PREFLIGHT=1` exists for test harnesses only.

After ANY resolver distress: restart the instance fresh (`unbound.sh
restart`) and never reuse a cache that witnessed the distress — poisoned
negative-cache entries outlive their cause by tens of minutes.

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
  <id>` so the journal is the source of truth. NEVER `--resume latest`
  for waves (resume hazard below): fresh wave passes run without
  `--resume`; a retry resumes explicitly from the wave's recorded run id
  (never blind-rescan, census guard 3).
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
  check, pass 1 (fresh — no `--resume`; retries resume explicitly from
  `<out-dir>/run-id`, captured after every attempt), a 30-min progress
  poller, the gate, and on health-FAIL the cleanup loop — all appended
  to `<out-dir>/monitor.log` so a detached wave survives its session.
  The poller logs a WARNING when the running unknown rate exceeds the
  D3 bar (env `GDD_WARN_PCT`, default `2` = percent; `0` disables). On
  completion (either path) it snapshots the journal (below).
- Gate coverage assertion: the runner always passes `--manifest` (and
  `--expect-skip` on retries, journal-derived), so a pass whose scanned
  + unparseable + expected skips do not reconcile with the manifest
  fails with exit 4 ("coverage gap") and skips cleanup — reconcile
  against the journal instead (resume hazard below).
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

### www-strip policy (decided 2026-11-12)

`domain.Parse` always strips one leading `www.` label, so manifest lines
like `www.edu.cn` scan as `edu.cn` — including when the stripped name IS
a public suffix. Policy: **normalize and scan.** A hosted public-suffix
apex has real DNS records (26 such cases in tranco1m-a scanned fine);
skipping them would add a skip class for zero benefit. Consequence,
handled: the gate's `--manifest` coverage check normalizes manifest
lines the same way (lowercase, trailing dot, leading `www.`) before
uniquing, so `www.foo.ch` + `foo.ch` in one manifest count as ONE target
— matching the feeder's dedup — instead of a false coverage gap.

### Resume hazard (2026-11-12 wave-A post-mortem)

`--resume latest` resolves to the most recently **started** run in the
journal — including aborted soaks and pilots — and silently skips every
domain that run journaled; the scan prints nothing about the skips.
Wave A's pass 1 ran while soak3 (aborted, 2 731 observations journaled)
was "latest": run 01M755NSBKB09MXM0EB0001CZB scanned 97 265 of 99 996
parseable domains (100 000 manifest − 4 unparseable). The health gate
passed (unknown 1.53% < 2%) because coverage is not a health metric;
the missing 2 731 were only found by reconciling run1.jsonl against
the manifest (input lines absent from the output: 2 761 = 2 731
skipped + 4 unparseable + 26 scanned under www-stripped names, e.g.
`www.edu.cn` → `edu.cn`, whose literal input lines also disappear).
The 2 731 domains exist in the journal
under soak3's run id with full evidence — coverage is recoverable by a
patch scan (owner call, wave B is held anyway).

Protections since: the gate's `--manifest` coverage assertion (exit 4),
the runner's fresh-pass/explicit-retry resume policy, and `run-id`
capture per attempt. The prior "90 834 denominators" in three reports
were never a real quantity (no journal or manifest figure reproduces
it) — first appeared underved 2026-11-11 14:44 and was copied onward.

### Wave ledger

| Wave             | Ranks             | List  | Manifest sha256 (first 12) | Status                                                          |
| ---------------- | ----------------- | ----- | -------------------------- | --------------------------------------------------------------- |
| tranco1m-a       | 1–100 000         | 947N2 | b4450f660567               | pass 1 done 2026-11-12 04:51 — health PASS, 97 265 scanned, unknown 1.53%; coverage gap 2 731 closed by patch (below) |
| tranco1m-a-patch | the 2 731 gap     | —     | derived from journal       | done 2026-11-12/13 — pass 1 + 5 cleanup rounds; final residual 15/2 731 error-class = 6 NXDOMAIN (domain truth) + 9 unknown (3 timeout, 6 SERVFAIL) = 0.33% < 0.5% bar on original-size accounting → UNDER BAR; round-relative gate verdicts on c4/c5 are by-design meaningless (round input = 100% prior errorers). 2026-11-13 nxdomain-fabrication finding makes the 6 NXDOMAINs + the forensics pilot's 946 re-verify candidates (see TODO_LIST nxdomain-bucket row) |
| tranco1m-b       | 100 001–500 000   | 947N2 | b6d5b3821c90               | pending — launch gated on a 5k pilot PASS through the TCP forwarder |
| tranco1m-c       | 500 001–1 000 000 | 947N2 | b0317d71873a               | pending — chains after B's gate PASS |

## Worker-pressure findings (E7/E9 ladder + 2026-11-11 soaks)

Two resolver modes were measured, cold cache, 10s timeout, mixed-TLD
Tranco slices (worst-case single-TLD .ch noted where it differs):

| Mode           | Workers | dom/s | unknown% | Gate |
| -------------- | ------- | ----- | -------- | ---- |
| full recursion | 2       | 2.9   | 3.0%     | FAIL |
| full recursion | 4       | 1.2   | 19.5–24% | FAIL |
| full recursion | 8       | 0.7   | 20–22%   | FAIL |
| forwarder      | 4       | 3.7   | 0.5%     | PASS |
| forwarder      | 8       | 4.1   | 1.5%     | PASS |
| forwarder      | 16      | 1.9   | 65.5%    | FAIL |

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
