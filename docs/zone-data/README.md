# Zone-Data Access — .uk / .de / .ch

Whole-TLD scanning needs the complete registered-domain population. Tranco
covers only the traffic-active subset (~63k of ~30M across the three TLDs).
Each registry gates its data behind paperwork only the account holder (Lars)
can sign; this directory carries the researched requirements and the drafted
applications so sending them is a 5-minute owner action, not a research
project.

| TLD | Registry | Program (verified 2026-11-10)              | Fee   | Draft           | Status             |
| --- | -------- | ------------------------------------------ | ----- | --------------- | ------------------ |
| .uk | Nominet  | .UK Zone Files Access Licence              | free  | `nominet-uk.md` | draft ready        |
| .de | DENIC    | (research pending)                         | (tbd) | `denic-de.md`   | researching        |
| .ch | SWITCH   | none — open data: public TSIG keys + AXFR   | free  | `switch-ch.md`  | ready — no owner action |

## Constraint review (what the licences permit)

All three programs are for RESEARCH/ANALYSIS of DNS-derived facts. The
detector reads only public DNS records (MX/TXT/CNAME/A/NS) for domains taken
from the zone files; it stores no registrant personal data. Licence terms
seen so far (Nominet) permit this: no redistribution of the zone data, no
high-volume queries against the REGISTRY's own systems (we query authoritative
DNS servers, not Nominet endpoints), secure storage, lawful use. Zone files
must not be republished; scan output (verdicts per domain) is derived
analysis, not the zone file — keep it that way in any published results.

## Owner checklist

1. .uk: send the email in `nominet-uk.md` to registrars@nominet.uk, then
   complete MiTek ID verification when invited; account details arrive via
   Files.com.
2. .ch: NO owner action needed — public TSIG keys and the exact dig
   command are in `switch-ch.md` (verified against zonedata.switch.ch);
   the only decision is whether the research-purpose clause covers the
   intended use.
3. .de: follow the steps in `denic-de.md` once research lands.
4. Track follow-ups in TODO_LIST.md ("Whole-TLD coverage" section).
