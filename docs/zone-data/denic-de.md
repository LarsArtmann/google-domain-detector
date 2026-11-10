# DENIC — .de Zone Data: NOT AVAILABLE to third parties

**Research completed 2026-11-10 (official DENIC pages, verified).** The
plan's "DENIC RDS application" premise was stale: no such program exists.
DENIC categorically refuses to hand .de zone files to ANY third party —
there is no application, no fee schedule, no delivery mechanism, and no
contact address for zone access because the offering does not exist.

## DENIC's official position (FAQ, verbatim)

> "The DENIC zone files contain – roughly spoken – a list of all
> .de-domains together with the information on the name servers that
> belong to them. **DENIC does not hand over these zone files (or parts
> of it) to any third parties.** Such disclosure would run counter to
> the imperatives of data protection and would undermine the defence of
> domain holders against possible abuse. Germany's Federal Data
> Protection Act is rather far-reaching in this respect."

They add a bank analogy ("No bank publishes a complete list of all the
account numbers it has issued") and note DENIC holds copyright on the
zone file. Their services list contains no zone-data offering; ICANN's
CZDS covers gTLDs only and explicitly defers ccTLDs to their registries.

## Consequences for whole-TLD coverage

- **.de full-coverage scanning via zone files is impossible.** Drop the
  application path; do not draft anything.
- Fallbacks for .de domain discovery (in decreasing completeness):
  1. **Tranco top-1M** (traffic-active subset — already used, 29,487 .de)
  2. **crt.sh certificate-log extractor** (subdomain-bearing .de names;
     the T19 fallback extractor covers .de alongside .ch)
  3. DENIC published statistics (counts only, no names)
- Legal posture unchanged: scanning public DNS of known domains is fine;
  the blocker is purely data acquisition.

## Sources (all fetched 2026-11-10)

- https://www.denic.de/en/faq/faqs-for-domain-holders/ (the verbatim
  refusal, question "Why does DENIC not make available the zone files
  for .de?")
- https://www.denic.de/en/services/ (services list: no zone-data program)
- https://czds.icann.org/ (gTLDs only)
