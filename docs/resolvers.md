# Choosing a resolver for bulk scans

The scanner talks DNS through `--resolver` (host or `host:port`; bare
IPv6 literals are bracketed automatically). Everything the tool concludes
depends on that hop telling the truth, and at bulk scale most resolvers
eventually don't. This doc is the decision guide; the wave machinery and
gate thresholds live in [census.md](census.md).

## TL;DR

| Scale                                        | Resolver                        | Workers / timeout |
| -------------------------------------------- | ------------------------------- | ----------------- |
| Up to a few hundred domains                  | system default is fine          | 8 / 5s            |
| Thousands and above                          | local unbound forwarder (below) | 4 / 10s           |
| Census / whole-TLD, or after any UDP trouble | TCP-upstream forwarder variant  | 4 / 10s           |

## What goes wrong, in order of scale

1. **Public resolvers (9.9.9.9, 1.1.1.1) throttle sustained bulk
   clients.** Two measured runs stayed at ~11–26% unknown through them.
   Fine for spot checks, wrong for batches.
2. **A local resolver in full-recursion mode collapses under parallel
   load.** Measured ladder (cold cache, 10s timeout): 2.9 dom/s at 2
   workers already 3% unknown; at 4+ workers, 20%+ unknown. Recursion
   mode is never the answer for bulk on one host.
3. **A local forwarder (unbound → Quad9/Cloudflare) is the sweet spot**:
   3.7 dom/s at 4 workers with 0.5% unknown. Above 8 workers unknown
   climbs again — the forwarder's upstream, not the scanner, saturates.
   `workers 4, timeout 10s` is the calibrated setting for every bulk run.
4. **Host UDP egress can degrade silently** (observed 2026-11-12:
   outbound UDP:53 dead for ~20 minutes while TCP:53 kept working), and
   a cache that served during distress keeps poisoned negative entries
   for tens of minutes after recovery. The TCP-upstream variant below is
   immune to both classes on the upstream hop.

## Running the local forwarder

```bash
scripts/unbound.sh start        # canonical UDP forwarder on 127.0.0.1:5353
GDD_UNBOUND_CONF=scripts/unbound-tcp.conf scripts/unbound.sh start   # TCP-upstream variant on :5354
gdd scan --resolver 127.0.0.1:5353 ...   # or :5354 for the TCP variant
```

- User-scoped, never root, never the system resolver. Lifecycle only via
  `scripts/unbound.sh` — never a bare `nohup unbound` (an orphaned second
  instance on the same port poisoned a real run; see AGENTS.md).
- One scan at a time through one instance: concurrent scans share the
  forwarder and poison each other's unknown-rate measurements.
- After ANY resolver distress (outage, overload, weird verdicts):
  `restart` the instance and never re-measure through a cache that
  watched the distress — stale negatives reproduce convincingly.
- The TCP-upstream variant (`tcp-upstream: yes`) moves only the
  unbound → upstream hop to TCP. The scanner → unbound hop stays UDP
  because Go's resolver cannot do DNS-over-TCP via its Dial callback
  (framing follows the network string Go passed). Use it when the host's
  UDP path is untrusted; it costs wall-clock, buys answer integrity.

## Verdict-truth reminders

- NXDOMAIN means "the name does not exist" only when it comes from the
  rcode — which is why the scanner consults an existence oracle before
  stamping it (Go's public API conflates NXDOMAIN with NODATA).
- A lying or distressed cache serves false negatives, never fabricated
  positives: positive verdicts survive resolver trouble, coverage
  claims don't.
- Before launching anything long: one real uncached lookup through the
  exact path the scan will use (`gdd doctor`), plus a direct upstream
  probe. A forwarder answering from cache proves nothing about upstream
  health.

## Memory at bulk scale

Buffered formats (table/json/csv) materialize the whole report (~290 B
per domain at 50k; ~700 MiB projected at 2.5M). Use `--format jsonl`
for anything large — it streams per result while the input feeder
already chunks at 4096 domains; a 50k-domain run measures ~40 MiB RSS
flat.
