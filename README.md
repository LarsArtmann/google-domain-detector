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
list survives a garbage row. `enrich` joins an event-sourced journal back
onto your CSV (`enrich --event-store scans.db --input leads.csv`),
appending the verdict columns while preserving every original column —
the round trip for CRM pipelines, no DNS in the join.

Formats: `--format table` (default), `json` (a JSON array carrying full
evidence per domain — verdicts, confidence, every record), `jsonl` (one
JSON object per line, written the moment each domain completes — the
choice for bulk runs; lines appear in completion order, not input order),
`csv`, `tsv`, `markdown`. Filters: `--only workspace` / `gcp` / `google`
/ `all`. Concurrency: `--workers` (default 8); per-domain budget
`--timeout` (default 5s). Colors: `--color auto|always|never` (`auto`
respects `NO_COLOR`; `--no-color` is the never shorthand). Bulk scans:
`--resolver host[:port]` queries one DNS server directly instead of the
system resolver (env `GDD_RESOLVER`). `scan --help` lists everything,
`describe` prints the machine-readable CLI contract, `doctor` preflights
the two external dependencies — that your resolver answers and the GCP
range feed loads — before a bulk run wastes minutes discovering it the
hard way, and interactive scans show live per-domain progress (pipes and
`--format jsonl` stay clean for scripting). Long runs can commit to a
whole-run budget with `--max-runtime 2h30m` — at the deadline partial
results still render, journal, and count; the stderr summary names how
many domains never got their turn.

Proven at bulk scale: the Tranco top-1M filtered to `.de`/`.ch`/`.uk`
(~63k domains) scans in ~20 minutes at `--workers 24`.

### Event-sourced scans (go-cqrs-lite)

`--event-store FILE.db` (or `GDD_EVENT_STORE`) persists every run as an
append-only event journal in SQLite — one stream per domain (re-scans keep
their history), one per run (started/completed with the verdict tally):

```bash
google-domain-detector scan --event-store scans.db --resolver 127.0.0.1:53 -j 24 domains.txt
google-domain-detector replay --event-store scans.db            # latest run, no DNS
google-domain-detector replay --event-store scans.db --run ID   # any earlier run
google-domain-detector show acme.com --event-store scans.db     # one domain's full history
google-domain-detector runs --event-store scans.db              # the run ledger
google-domain-detector replay --event-store scans.db --audit-excluded ID1,ID2   # poison check (wave names work too)
google-domain-detector scan --resume latest --event-store scans.db domains.txt  # skip what's done
```

A multi-hour run that dies is NOT lost work: `--resume` skips every domain
the journal already observed and links the new run to the old one.

`replay` rebuilds the report purely from the journal — same results, same
evidence, zero DNS (3645 domains replay in ~80 ms; pinned by a test). Scan
output preserves input order, replay preserves journal (completion) order —
same facts, different orderings. Built on the
[go-cqrs-lite](https://github.com/LarsArtmann/go-cqrs-lite) `system`
composition root with metaengine projections; events carry evidence only —
verdicts are always recomputed from it.

### Deltas: who adopted or left Google (`--since`)

The journal's killer feature. Re-scan the same list later, then diff the
runs — the domains whose verdict flipped are the story:

```bash
google-domain-detector scan  --event-store scans.db --resolver 127.0.0.1:53 -j 8 domains.txt   # next week
google-domain-detector replay --event-store scans.db --since <previous-run-id>                  # flips only
```

`--since` reports exactly the domains whose Workspace or GCP detection
changed since the named run, with adopted/left counts on stderr. Errored
lookups are treated as unknown, never as a departure — a domain that
timed out this week did not leave Google. Flips across different
detection rulesets (each run stamps its ruleset) get an explicit warning:
rule changes can masquerade as adoption. Works with every `--format`,
including streaming `jsonl`.

### Trusting a journal: the supersession audit (`--audit-excluded`)

A long-lived journal accumulates runs you may distrust (a diagnostic
against the wrong binary, a pilot through the system resolver). Verdicts
are last-observation-wins, so "the bad run's verdicts are superseded" is
a per-domain claim — the audit proves it mechanically:

```bash
google-domain-detector replay --event-store scans.db --audit-excluded <run-id-or-wave>[,<...>]
```

lists every domain whose latest observation comes from an excluded run
(a run ID, a wave name, or `wave:NAME` — a wave expands to every run
stamped with it). Findings are data, not failure: exit 0 either way, the
stderr count is what a gate asserts. Unknown run IDs or wave names fail
the audit instead of silently reporting clean.

## Honest limitations

- **Only externally visible services are detectable.** A backend on Cloud Run
  behind a Cloudflare-fronted custom domain resolves to Cloudflare IPs and
  shows nothing. The scan proves presence, never absence.
- **Google IP ranges** are checked against `cloud.json` (GCP only) — a domain
  on Google's corporate space (e.g. `google.com` itself) is not flagged GCP
  by address. That is deliberate: the question is GCP, not Google-anywhere.
- **Mail gateways fronting Workspace** (Mimecast, Proofpoint) hide the MX
  signal; the SPF include and the DKIM probe still catch most of them.
- **Error tails are resolver-dependent, never ground truth.** A hiccup
  surfaces as an `Error` cell. NXDOMAIN verdicts are rcode-level truth
  since 2026-11-13 (the scanner used to conflate Go's NXDOMAIN/NODATA
  `IsNotFound` into domain death, stamping ~90% of one census wave's NX
  verdicts false) — but a resolver serving stale cached negatives can
  still lie, so treat every `Error` cell as "unknown, ask again via a
  fresh `--resolver`", not as a fact about the domain.

## Install

```bash
go install github.com/larsartmann/google-domain-detector/cmd/google-domain-detector@latest
```

or via Homebrew:

```bash
brew install LarsArtmann/tap/google-domain-detector
```

Requires Go ≥ 1.27 (binary formula runs without it). Nix users get a
hermetic build from the repo flake (`nix build`; the binary lands in
`./result/bin/`).

The range feed fetch identifies itself as
`User-Agent: google-domain-detector/<version>` — a courtesy to Google's
feed so bulk users are distinguishable from scripts pretending to be
browsers.

## Build & test

```bash
go build ./...     # build
go test ./...      # offline test suite (fake DNS, httptest feeds)
nix build          # hermetic build via go-nix-helpers
```

Go ≥ 1.27. Quality gate: [BuildFlow](https://github.com/LarsArtmann/BuildFlow)
(`buildflow`), foundation decisions in [STACK.md](STACK.md).

MIT — see [LICENSE](LICENSE).
