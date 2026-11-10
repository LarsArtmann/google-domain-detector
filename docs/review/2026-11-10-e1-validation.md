# E1 — validation run #3 (replication, 2026-11-10 10:10–10:22 CET)

Binary at HEAD `e712a1f`-era master. 3 645-domain `.ch` slice (same input as
the 07-46 T5 run), system resolver, workers 8, timeout 8s, event store on.

## Results

- `certain 336 / likely 142, gcp 74` — errors 739/3645 (20.3%):
  nxdomain 597, timeout 40, servfail 102
- ELAPSED 762s, MAXRSS 64MB
- Journal replay content-identical: 3645 = 3645, **zero diffs**
  (sorted full-object compare, run3-scan.json vs run3-replay.json)

## Cross-checks

- 10-verdict dig sample via Quad9 (run3-dig-sample.txt): **5/5 positive
  verdicts confirmed** (aspmx.l.google.com MX for the certain; 34.x/199.36.x
  Google IPs for both GCP; likely = SPF-class signals without MX, rule
  respected). PLAIN samples correctly non-Google (Mimecast/Cloudflare/own MX).
- NX bucket sample (run3-nx-sample.txt): **11/12 ALIVE** — own SOA + NS
  delegation present (uka.ch, truck1.ch, blogblick.ch, …); juwelierkeller.ch
  the only true dead. uka.ch ground truth: authoritative NS, Quad9, and the
  local resolver ALL answer `194.150.248.53` (NOERROR) 20 minutes after the
  run recorded it NXDOMAIN.

## Finding (supersedes the 07-46 "~90% genuinely dead" tail conclusion)

The local resolver's NEGATIVE CACHE entries from the morning's distressed
runs outlived the distress and lied NXDOMAIN for live domains throughout
this run. The 07-46 re-verification (tool re-scan via Quad9 minutes after
poisoning) reproduced the same stale negatives — its ~89.6%-dead number
measured cache state, not domain liveness. Consequences:

1. Positive verdicts: trustworthy (never fabricated; 14/14 dig-confirmed).
2. NXDOMAIN-class results through a shared cache that witnessed distress:
   unreliable — assume most are false until re-asked via a fresh resolver.
3. Census mitigation stands (unbound, fresh cache, E7/C05); D1
   (never-retry-NXDOMAIN) unchanged; D3's tail bar can only be assessed on
   an unbound-backed run.

Artifacts: gdd, run3.db (+wal/shm), run3-scan.json/.stderr, run3-replay.json/.stderr,
run3-dig-sample.txt, run3-nx-sample.txt.
