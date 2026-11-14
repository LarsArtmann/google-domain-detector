# ROADMAP

Long-term direction and raw ideas not yet refined into actionable tasks.
Actionable short-term work lives in [TODO_LIST](TODO_LIST.md); feature
inventory in [FEATURES](FEATURES.md).

## Direction

1. **The census** (canonical plan: docs/planning/2026-11-10_06-18_*): the
   Tranco top-1M scanned through the user-scoped unbound resolver, in
   gated waves, journaled end to end, producing per-TLD coverage numbers
   and the verdict-flip delta feed — the sales signal.
2. **Whole-TLD reach**: .ch via the verified SWITCH TSIG pull (input
   ready), .uk via the Nominet DAA application (owner draft ready),
   .de via Tranco + the daily crt.sh fallback (DENIC refuses zone
   access outright — resolved-impossible 2026-11-10).
3. **Daily-driver UX**: doctor preflights (done), live progress (done),
   scoped exit codes (done), delta query (done); next: daily delta
   timer, lead scoring.

## Parked ideas (not refined — do not treat as commitments)

- **CAA records as a Workspace fingerprint** — only if real-world evidence
  emerges (e.g. `caa record 0 issue "pki.goog"` correlates); no speculative
  probes (2026-11-10 decision).
- **Journal compaction** for multi-million-event stores: snapshot
  generation + tail-replay when a census db passes a few GiB. The 2.5M
  extrapolation (~3 GiB) says this can wait until after wave C.
- **Lead scoring v1**: rank workspace-certain × GCP-present × Tranco-rank
  into a scored lead list; needs a census run to tune weights.
- **DNSSEC-aware NXDOMAIN confidence**: if a resolver returns an authenticated
  denial of existence (AD bit with NSEC), an NXDOMAIN could be upgraded
  from "cache-dependent claim" to "verified fact" — needs the own-recursive
  resolver from the census to be worth prototyping.
- **Enrichment join**: imprint/contact crawl for the top-N matches
  (legal care: robots/ToS) or company-master-data cross-ref — post-census
  (06-09 §f12).
- **Dedup by registrant/organization** for portfolio-owned domain clusters
  — needs zone/WHOIS data first (06-09 §f13).
- **Prospects output mode**: domains → company-plausible names for
  outreach lists — needs scoping before anyone promises it
  (19-27 §f43); the detector proves *domains*, not companies.
- **GoReleaser binaries** — cross-platform release binaries beyond the
  nix + Homebrew-tap paths (E28 idea, 2026-11-11).
- **Verdict-flip alerting** (daily digest of delta-query output) — the
  delta query exists (`replay --since`); the digest/timer is unrefined.
