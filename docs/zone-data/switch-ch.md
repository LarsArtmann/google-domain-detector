# SWITCH — .ch / .li Zone File (open data, no application)

**Research delivered 2026-11-10 (source: www.switch.ch contact page).**
Unlike DENIC and Nominet, SWITCH does NOT gate the .ch/.li zone file behind
an application: there is no zone-data application program, no forms, no
fees. The zone is open data under the switch.ch Acceptable Use Policy,
published via standard DNS zone transfer (AXFR) with public TSIG keys.

## How to obtain it

- Request the TSIG keys at `tsig@switch.ch` (free).
- Fetch the zone with an AXFR-capable client (e.g. `ldns-axfr`,
  `dig AXFR`) against the .ch / .li nameservers, keyed with those TSIG
  credentials:
  - `dig -y '<tsig-name>:<tsig-secret>' AXFR ch @ns1.ch`
- That yields the complete .ch zone (≈2.4M names); filter to owner names,
  dedupe, feed the scanner via `--input`.

## Licence constraints (Acceptable Use Policy, switch.ch)

Standard fair-use: no abuse of SWITCH infrastructure, no unlawful use. The
zone itself is published as open data; derived scan analysis is fine. Do not
republish the raw zone.

## Steps

1. Email tsig@switch.ch requesting the TSIG key (owner action — a
   formality, not an application).
2. AXFR the .ch and .li zones.
3. Extract registered names → `--input ch-domains.txt` → the scanner's
   whole-TLD path (`docs/planning/2026-11-10_00-50_*`, tail tasks).

## Source

- https://www.switch.ch/contact/ (states zone file availability via AXFR
  with TSIG keys from tsig@switch.ch, fetched 2026-11-10)
