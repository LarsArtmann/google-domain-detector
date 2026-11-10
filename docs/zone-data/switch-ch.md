# SWITCH — .ch / .li Zone File (open data, no application)

**Verified 2026-11-10 against https://zonedata.switch.ch/ (SWITCH Open
Data portal).** Unlike DENIC and Nominet, SWITCH gates nothing: no
application, no forms, no fees, no email request. The .ch and .li zone
files (all delegated domain names) are published as open data, fetched
via standard DNS zone transfer (AXFR) from `zonedata.switch.ch` using
TSIG keys that are PUBLIC — printed directly on the page.

## How to obtain it

1. Save the public TSIG key (from https://zonedata.switch.ch/) to
   `ch_zonedata.key`:

   ```
   key tsig-zonedata-ch-public-21-01 {
       algorithm hmac-sha512;
       secret "stZwEGApYumtXkh73qMLPqfbIDozWKZLkqRvcjKSpRnsor6A6MxixRL6C2HeSVBQNfMW4wer+qjS0ZSfiWiJ3Q==";
   };
   ```

   (analogous `li_zonedata.key` with
   `tsig-zonedata-li-public-21-01` for .li)

2. Transfer the zone (exact command from the SWITCH page):

   ```
   dig -k ch_zonedata.key @zonedata.switch.ch +noall +answer +noidnout +onesoa AXFR ch. > ch.txt
   ```

3. That yields the complete .ch zone (≈2.4M names); filter to owner
   names, dedupe, feed the scanner via `--input`.

## Usage constraints (binding on download)

By downloading you agree (per Article 10(1)(a)(6) of the Swiss
[Ordinance on Internet Domains](https://www.admin.ch/opc/de/classified-compilation/20141744/index.html)):

- Use only for **combating cybercrime, scientific and social research,
  or other purposes in the public interest**. Technographics research on
  DNS-derived facts fits; check the purpose clause before commercial
  lead-gen use of .ch zone data.
- **Download each zone file at most once every 24 h** (no DNS notify
  service exists; polling more often is abuse).
- Abuse or legal-term violations → SWITCH reserves the right to block
  the offending IP prefix.
- Do not republish the raw zone; scan output (verdicts per domain) is
  derived analysis — keep it that way in anything published.
- Questions: Switch CERT — dns-operation@switch.ch, +41 44 268 15 15.

## Bonus datasets on the same portal

- **Top 1000 .ch** (monthly CSV since 2018, ranked by distinct ASes):
  https://zonedata.switch.ch/open-data/top1000/latest.csv
- **DNS Health Report** (daily JSONL.gz scans of .ch/.li NS compliance)

## Status

No owner action required beyond running the dig command — the only
decision is whether the research-purpose clause covers the intended use.

## Source

- https://zonedata.switch.ch/ (fetched 2026-11-10; keys, dig command,
  ordinance link, and constraints quoted verbatim from the page)
