# google-domain-detector

**Which companies run Google Workspace or Google Cloud? Ask their DNS.**

Feed it a list of domains — arguments, a file, a CSV export from your CRM, a
piped list — and it reports, per domain, whether public DNS shows Google
Workspace and/or Google Cloud Platform usage, with the exact records that
prove it. No API keys, no cost, no accounts: everything it checks is a public
DNS record.

```bash
# A few domains from the command line
google-domain-detector scan artmann.tech golang.org cloudflare.com

# A CRM export with a "Website" column
google-domain-detector scan --input leads.csv --only workspace

# Piped, JSON out, into jq
echo "artmann.tech" | google-domain-detector scan --format json | jq '.[] | select(.workspace.detected)'
```

## What it detects

| Signal (DNS)                                                                                | Verdict       | Confidence                                       |
| ------------------------------------------------------------------------------------------- | ------------- | ------------------------------------------------ |
| MX hosts under `google.com` / `googlemail.com` (`smtp.google.com`, `aspmx.l.google.com`, …) | **Workspace** | certain — mail is delivered to Google            |
| SPF `include:_spf.google.com`                                                               | **Workspace** | likely — free Gmail "send as" also produces this |
| TXT at `google._domainkey.<domain>` (probed only when MX+SPF are silent)                    | **Workspace** | likely                                           |
| CNAME to `*.run.app` (Cloud Run), `*.appspot.com` (App Engine)                              | **GCP**       | certain                                          |
| CNAME to `*.web.app` / `*.firebaseapp.com` (Firebase Hosting)                               | **GCP**       | certain                                          |
| CNAME to `*.cloudfunctions.net`, `*.googlehosted.com`, `*.storage.googleapis.com`           | **GCP**       | certain                                          |
| NS under `*.googledomains.com` (Cloud DNS)                                                  | **GCP**       | certain                                          |
| A/AAAA inside Google's published GCP ranges (`gstatic.com/ipranges/cloud.json`)             | **GCP**       | certain                                          |

`google-site-verification=` TXT records are reported as generic Google usage
but never drive a platform verdict.

Apex and `www` are both probed; ranges are fetched once per run and cached
for 24 h (`~/.cache/google-domain-detector/cloud.json`, stale cache is the
fallback when the feed is unreachable).

## Input & output

Input modes (combined freely, first-seen order, deduplicated):

- arguments: bare domains or URLs
- `--input FILE`: plain list (one or more domains/URLs per line) or CSV —
  the first column named `domain`, `domains`, `website`, `url`, `host`,
  `hostname`, `site`, or `web` is used
- `--input -` or piped stdin

Unparseable entries are skipped and counted, never fatal — a 500-row lead
list survives a garbage row.

Formats: `--format table` (default), `json` (full evidence, one line per
domain, jq-friendly), `csv`, `tsv`, `markdown`. Filters: `--only workspace`
/ `gcp` / `google` / `all`. Concurrency: `--workers` (default 8); per-domain
budget `--timeout` (default 5s). Bulk scans: `--resolver host[:port]`
queries one DNS server directly instead of the system resolver (env
`GDD_RESOLVER`). `scan --help` lists everything, `describe`
prints the machine-readable CLI contract.

Proven at bulk scale: the Tranco top-1M filtered to `.de`/`.ch`/`.uk`
(~63k domains) scans in ~20 minutes at `--workers 24`.

## Honest limitations

- **Only externally visible services are detectable.** A backend on Cloud Run
  behind a Cloudflare-fronted custom domain resolves to Cloudflare IPs and
  shows nothing. The scan proves presence, never absence.
- **Google IP ranges** are checked against `cloud.json` (GCP only) — a domain
  on Google's corporate space (e.g. `google.com` itself) is not flagged GCP
  by address. That is deliberate: the question is GCP, not Google-anywhere.
- **Mail gateways fronting Workspace** (Mimecast, Proofpoint) hide the MX
  signal; the SPF include and the DKIM probe still catch most of them.
- A local resolver hiccup surfaces as an `Error` cell on that domain —
  rerun rather than trust a partially failed batch silently.

## Build & test

```bash
go build ./...     # build
go test ./...      # offline test suite (fake DNS, httptest feeds)
nix build          # hermetic build via go-nix-helpers
```

Go ≥ 1.27. Quality gate: [BuildFlow](https://github.com/LarsArtmann/BuildFlow)
(`buildflow`), foundation decisions in [STACK.md](STACK.md).

MIT — see [LICENSE](LICENSE).
