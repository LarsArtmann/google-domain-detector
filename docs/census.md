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

### The false-NXDOMAIN root cause and the TCP-upstream variant (2026-11-13)

Two independent findings, only one of which was ever a resolver defect:

1. **Scanner-side NODATA/NXDOMAIN conflation (THE root cause — fixed).**
   Go's resolver maps "name does not exist" (rcode NXDOMAIN) AND "name
   exists but has no such record" (NODATA) into the SAME `DNSError` with
   `IsNotFound` — indistinguishable through the public API (verified
   against the Go 1.27 toolchain source: both paths construct
   `errNoSuchHost`). `scan.ScanDomain` treated any `IsNotFound` from the
   apex TXT/MX/NS probes as an NXDOMAIN verdict, so EVERY live domain
   with no MX record and no Google-relevant evidence (no SPF, no
   google-site-verification, no googledomains.com NS, no Google CNAME)
   was stamped `error_kind=nxdomain` — deterministic, worker-count
   independent, and permanent (NX is domain truth, never retried).
   Evidence chain: patch cleanup round 4 produced 6 NXDOMAIN verdicts
   that `dig @9.9.9.9` answered NOERROR (3 proven to have live A
   records); they reproduced IDENTICALLY through a fresh-cache
   TCP-upstream instance in isolation — ruling out any resolver path —
   and a pure-stdlib probe showed the MX lookup itself returning
   IsNotFound while TXT succeeded. 27 of 30 sampled wave-A "NXDOMAIN"
   domains are alive at Quad9 (windowsupdate.com, aliexpress.us,
   pp.ua, linodeobjects.com): the wave-A NX bucket (22,010 domains,
   22.6%) is ~90% false. The earlier "the forwarder fabricates
   NXDOMAINs under load" theory (status report 01-04 §) is FALSIFIED —
   the falsifying experiment and the fix are in
   `docs/status/2026-11-13_*` (existence-oracle session).
   **Fix (2026-11-13):** `resolve.LookupExistence` — a minimal DNS wire
   client (A query, header rcode, TC→TCP fallback) that returns
   Exists/NotExists/error with NODATA mapped to Exists; ScanDomain
   consults it FIRST (NotExists = true NXDOMAIN, short-circuit) and
   treats sub-lookup IsNotFound as ordinary record absence. Detection
   rules unchanged, ruleset still v1. **Correction wave
   `tranco1m-a-nxfix`** re-scans the poisoned NX bucket
   (22,136 domains = wave A's 22,010 + the patch wave's 126, disjoint)
   under the fixed scanner; replay last-wins superseds the false
   verdicts.
2. **Host-level outbound UDP:53 outage (2026-11-12 23:24–23:45).**
   Every fresh UDP recursion died (`dig @9.9.9.9` = no servers
   reachable) while TCP:53 and ICMP kept working; a wave launched into
   that window fabricated ~3,450 garbage observations (SERVFAIL-class —
   correctable, unlike false NX).

Operational note, no proven causation: the unbound log warns
`cannot increase max open fds from 65536 to 131566 — continuing with
less udp ports: 4065` (the 8192 outgoing-range wants more fds than the
user-scoped ulimit allows). It was suspected in the NX fabrication
theory above; with that theory falsified it remains just a capacity
warning.

**Outage mitigation: `scripts/unbound-tcp.conf` — TCP-upstream
forwarding on `127.0.0.1:5354`** (`tcp-upstream: yes`: the unbound →
upstream hop is TCP end to end, immune to a UDP-only outage; the
scanner → unbound hop stays loopback UDP — Go's resolver can't do
DNS-over-TCP via its Dial callback without breaking framing, so
scanner-side TCP is not an option). Verified end-to-end 2026-11-13
(gdd doctor 4/4 through 5354; the six former false-NX specimens scan
clean through it). Cost: wall-clock — per-query upstream connections
are slower than UDP; a fresh 5k pilot through this config measures the
throughput and is the launch gate for any wave on it. Usage
(per-instance lifecycle, never bare nohup):

```bash
GDD_UNBOUND_CONF=scripts/unbound-tcp.conf scripts/unbound.sh start   # also: status | stop | restart
# scans then target it with --resolver 127.0.0.1:5354
```

### G4 journal capacity + sink/cancel semantics (2026-11-10, plan v3)

The journal is trustworthy to at least 100k events — append 18.7 s
(5 345/s), replay 2.0 s (51 035/s), db 125 MiB, heap 15 MiB
(`TestJournalOneHundredThousandEvents`, opt-in via `GDD_SOAK=1` — the soak
exceeds CI's default 10-min budget on 2-core runners and is not a
functional gate). Extrapolated to 2.5M domains: ~8 min append, ~3 GiB db,
~50 s replay. The sink retries each observation 3× with backoff and
reports the exact dropped count on Close. Ctrl-C mid-domain drains and
still completes the run record (pinned by `TestScanCancelMidRunDrainsEventStore`
with a deterministic cancel trigger — never wall-clock `AfterFunc`, which
raced suite load); a cancel landing during event-store startup fails the
run cleanly with exit 1, nothing journaled — correct behavior.

Negative-cache lesson from the 2026-11-10/12 incident (the resolver was
falsified as the false-NX cause, but the operational tail stands):
negative-cache entries PERSIST tens of minutes past their cause — never
re-scan a poisoned bucket through the cache that watched the poisoning
(the retracted "~90% genuinely dead" claim measured stale negatives).
Positive verdicts were never affected (false negatives only).

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
  exist". Counted in coverage; never retried (owner default D1). Since
  2026-11-13 an NXDOMAIN verdict comes ONLY from the existence oracle
  (rcode-level) — Go's `IsNotFound` additionally covers NODATA ("name
  exists, no such record"), which is NOT domain truth; a no-MX live
  domain must scan clean, not die (see "false-NXDOMAIN root cause"
  above).
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
- Wave watchdog: every wave launches with an EXTERNAL detector detached
  next to it — `setsid nohup scripts/census-watchdog.sh <out-dir>
  >> <out-dir>/watchdog.log 2>&1 &`. It polls run1.jsonl growth (300 s);
  3 stagnant polls with no "pass 1 finished" line = WEDGED runner (exit 1,
  wave-A 2026-11-13 signature: scan done, gate/snapshot never ran) or DEAD
  runner (exit 2), logged as a WATCHDOG ALARM into monitor.log. The
  wrapper's own poller additionally logs HEARTBEAT STALL after 2 stagnant
  30-min polls (belt to the watchdog's braces). Root cause of the 08:43
  wedge: census-wave.sh edited on disk mid-run (inode offset shift →
  syntax error at pass-1 completion) — never edit the wrapper or ANY
  running bash script in place; build in /tmp and `install` over the path.
- Gate coverage assertion: the runner always passes `--manifest` (and
  `--expect-skip` on retries, journal-derived), so a pass whose scanned
  - unparseable + expected skips do not reconcile with the manifest
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

| Wave                 | Ranks                                 | List  | Manifest sha256 (first 12) | Status                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                         |
| -------------------- | ------------------------------------- | ----- | -------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| tranco1m-a           | 1–100 000                             | 947N2 | b4450f660567               | pass 1 done 2026-11-12 04:51 — health PASS, 97 265 scanned, unknown 1.53%; coverage gap 2 731 closed by patch (below)                                                                                                                                                                                                                                                                                                                                                                                                                                          |
| tranco1m-a-patch     | the 2 731 gap                         | —     | derived from journal       | done 2026-11-12/13 — pass 1 + 5 cleanup rounds; final residual 15/2 731 error-class = 6 NXDOMAIN + 9 unknown (3 timeout, 6 SERVFAIL). 2026-11-13 ROOT CAUSE: those 6 NXDOMAINs were scanner NODATA-conflation artifacts (3 proven live at Quad9), NOT domain truth — superseded by the tranco1m-a-nxfix correction wave below                                                                                                                                                                                                                                  |
| tranco1m-a-nxfix     | the 22,136 false-NX pool              | —     | 472fffefcdd2               | **done 2026-11-13 08:43** — pass 1 (22,136 scanned: 21,561 clean 97.3%, 365 NX 1.65%, 210 unknown 0.95%) + cleanup round 1 (210 → 92 = **0.42% < 0.5% bar → PASS**); post-fix NX verdicts 19/20 sample-confirmed at Quad9 (1 = q9 timeout). Journal run 01M7969W0M73G7SKHTQ4000001 (+ `-c1`); snapshot tranco1m-a-nxfix-complete-20261113-0843. NOTE: the runner died mid-run (census-wave.sh edited on disk while the wave bash executed → offset shift → syntax error at pass-1 completion); stages completed manually — see AGENTS.md gotcha + 02-05 report |
| tranco1m-a-poisonfix | the poisoned-supersession domains | —     | audit-derived              | **prepared 2026-11-13 16:45, awaiting B's single-flight + owner go** — input SHRUNK by the first mechanical audit run from 948 to **25 domains**: pilot 0 + soak3 0 (nxfix/good pass superseded them) + c4 10 + c5 15 (the residual patch tail incl. the 6 false-NX outside the nxfix manifest); list `reports/census/waves/tranco1m-a-poisonfix.txt` = `replay --audit-excluded 01M74VNNCJS6…,01M74QS4AY1M…,01M794AXA8YV…,01M794ZEB4HY…` jsonl output verbatim. The mechanical proof it worked: the same audit must report 0 poisoned after the pass (§Replay audit)                                                                                                                                                                                                                                                                                                                                                                                       |
| tranco1m-b | 100 001–500 000 | 947N2 | b6d5b3821c90               | **running 2026-11-13 16:53 (attempt 2, explicit resume)** — attempt 1 (08:45, gdd-20261113) wedged at 11:52:49: a hostile-TXT result (bide-et-musique.com, invalid UTF-8) broke the JSON v2 render → stdout froze; journal writes stalled ~16:13 (SQLITE_BUSY class); the NEW census-watchdog alarmed 16:22; SIGINT drained cleanly 16:46 (run `01M79XW7SF918E61NYVM000001`, 96,215 domains journaled; coverage-gap gate + snapshot 16:47). Attempt 2 on gdd-20261113b (scan-boundary UTF-8 sanitizer + CBOR decode repair, PROVENANCE.md): `--resume 01M79XW7SF918E61NYVM000001` explicit, expect-skip 96,215, w4/10s via 5354, ETA ≈ 24–30 h for the remaining ~304k |
| tranco1m-c           | 500 001–1 000 000                     | 947N2 | b0317d71873a               | pending — chains after B's gate PASS                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                           |

### Supersession audit (replay --audit-excluded, added 2026-11-13)

"Replay last-wins supersedes the false verdicts" is a per-domain claim.
The audit makes it mechanical: name the run IDs you distrust and it
lists every domain whose LAST journal observation comes from one of
them — exactly the input a correction pass must cover, and exactly the
check that it worked afterwards (expect `0 poisoned` on a clean
journal).

```bash
gdd replay --event-store census.db --audit-excluded RUN1,RUN2
gdd replay --event-store census.db --audit-excluded RUN1 --format jsonl   # one finding per line
```

Findings are data, not failure: exit 0 either way, gates assert the
stderr count. Unknown run IDs FAIL the audit — a typo must not report
clean. The 2026-11-13 self-review found two supersession gaps by hand
(the 6 c4/c5 false-NX outside the nxfix manifest; the pilot's ~946 NX
superseding healthy wave-A verdicts, §d2/d3 of the 08-57 report) —
this command is that check, never hand-rolled again.

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
inflated unknowns). Before launching: `pgrep -f 'gdd[^ ]* scan'` must be
empty (the pattern tolerates dated binaries like `gdd-20261113`), and the
wave lane owns long runs. Bulk scans also NEVER use the
system resolver directly — a 2026-11-11 pilot accidentally did (its
observations went through 127.0.0.1:53 and are excluded from census
accounting as protocol-contaminated). **Diagnostic scans write to
throwaway event-stores, always (2026-11-13):** a falsification
experiment stamped 6 false NX + 9 unknown into census.db under wave
`tranco1m-a-patch-c5`, and the journal carried that false domain-truth
until the poisonfix pass — a diagnostic that might record lies does not
belong in the production journal.

Single-TLD scans (.ch zone pulls): the same ladder's recursion-mode w2
row is the relevant bound — workers ≤ 2 plus cleanup loops, always.

**TCP-upstream forwarder (127.0.0.1:5354), measured 2026-11-13:** the
nxfix wave sustained 4.0–4.6 dom/s at w4/10s over 22 136 domains, and
wave B confirmed the same rate early. This is the calibrated bulk path;
the UDP forwarder rows above are older measurements.
