# DOMAIN_LANGUAGE.md — the census's ubiquitous language

One glossary, one meaning per term. When code, docs, and reports
disagree, this file wins and the code gets fixed. Sources: the type
definitions in `internal/classify`, `internal/scan`, `internal/scanstore`,
and the census runbook (`docs/census.md`).

## The domain in one paragraph

A **scan** feeds **domains** through DNS **lookups**, turns the answers
into **evidence**, derives **verdicts** from evidence, and records every
observation as an immutable **event** in an append-only **journal** — so
any past run can be **replayed** and any correction can be proven
against what was actually recorded.

## Core nouns

| Term        | Meaning                                                                                                                                                                                                                                                                                                                                                                                                       | Lives in                     |
| ----------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ---------------------------- |
| Domain      | One input hostname, normalized (URLs stripped, ports dropped, www kept as-is per the 2026-11-12 policy, punycode decoded in).                                                                                                                                                                                                                                                                                 | `internal/domain`            |
| Scan (run)  | One execution over one input list; identified by a 26-char ULID **run ID**. A run has a lifecycle: started → (resumed) → completed.                                                                                                                                                                                                                                                                           | `internal/scan`, `scanstore` |
| Wave        | A named batch of the census (e.g. `tranco1m-b`); a run is STAMPED with its wave. Waves group runs in the ledger and in audit exclusions.                                                                                                                                                                                                                                                                      | ops/journal stamp            |
| Observation | Everything one scan learned about one domain in one run — evidence, errors, provenance. Stored per-domain (stable stream), last-wins within a run.                                                                                                                                                                                                                                                            | `DomainObservedPayload`      |
| Evidence    | One DNS-derived fact: a **kind** (`mx_google`, `spf_google`, `dkim_google`, `mx_microsoft`, `spf_microsoft`, `cname_run_app`, `cname_appspot`, `cname_firebase`, `cname_cloudfunctions`, `cname_googlehosted`, `cname_gcs`, `ns_clouddns`, `ip_gcp_range`, `google_site_verification`), a **platform** (`workspace`, `gcp`, `google`, `microsoft`), a **weight** (`strong`, `weak`), the host and raw record. | `classify.Evidence`          |
| Verdict     | The DERIVED classification of a domain — never stored, always computed from evidence: Workspace **certain** (strong MX) / **likely** (weak SPF/DKIM), GCP certain (IP in cloud.json). Free-Gmail "send as" produces weak evidence only — it can never upgrade a Workspace verdict.                                                                                                                            | `classify.Result` methods    |
| Result      | Evidence list + error of one domain's scan: the raw material verdicts derive from.                                                                                                                                                                                                                                                                                                                            | `classify.Result`            |
| Counts      | A run's tally: scanned, per-verdict counts, errors by kind. Scanner-computed, journaled at completion.                                                                                                                                                                                                                                                                                                        | `classify.Counts`            |
| ErrorKind   | The taxonomy of a failed domain lookup: `nxdomain`, `timeout`, `servfail`, `canceled`, `other`. An errored domain is an **unknown**, never a dead domain.                                                                                                                                                                                                                                                     | `classify.ErrorKind`         |
| Unknown     | A domain whose observation errored — excluded from verdict denominators, retried by cleanup loops, never counted as "no Google usage".                                                                                                                                                                                                                                                                        | ops vocabulary               |

## Event-sourcing terms

| Term                    | Meaning                                                                                                                                                                                            |
| ----------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Journal                 | The append-only event store (SQLite via go-cqrs-lite metaengine). The only source of truth; read models are derivable.                                                                             |
| Stream                  | One event stream: per-domain (all observations of that domain) or per-run (started/completed).                                                                                                     |
| Ruleset                 | The frozen detection-rule set a run ran under (`v1-2026-11-10`, …), stamped into every event as provenance. Cross-runset deltas must warn.                                                         |
| Replay                  | Rebuilding a run's report purely from the journal — no DNS, no projections. Byte-level: input order preserved in scan output, journal order in replay.                                             |
| Read model / projection | A derived view folded from events (run ledger, observation counts). Not truth; rebuilt at will.                                                                                                    |
| Supersession            | A later run's observation REPLACING an earlier run's as the domain's current answer (last-wins per domain across runs).                                                                            |
| Poisoned supersession   | A supersession whose last observation came from a KNOWN-BAD run (e.g. the resolver-distress runs) — the audit (`replay --audit-excluded`) lists them; correction passes must drive the count to 0. |
| Schema version          | Every event carries one (`event.WithSchemaVersion(1)`); evolution upcasts, never rewrites.                                                                                                         |

## Census-ops terms

| Term               | Meaning                                                                                                                                             |
| ------------------ | --------------------------------------------------------------------------------------------------------------------------------------------------- |
| Single-flight      | Exactly ONE `gdd scan` at a time — concurrent scans share the forwarder and poison each other's measurements.                                       |
| Forwarder          | The user-scoped unbound caching resolver (UDP client hop 127.0.0.1:5353 / TCP-upstream 5354) all bulk scans go through — never the system resolver. |
| Gate               | The post-wave pass/fail check (`census-gate.sh`) on a wave's unknown rate against its manifest (`--expect-skip`).                                   |
| Cleanup loop (T14) | The per-wave retry pass for unknowns, armed on a gate FAIL, target < 0.5%.                                                                          |
| Snapshot           | A WAL-consistent copy of the journal (`sqlite3 .backup`), tarred + sha256'd after each wave; restore-verified before trusted.                       |
| Diagnostic store   | A THROWAWAY event store for falsification experiments — a diagnostic that might record lies never writes to the production journal.                 |
| none-class         | Observations with zero evidence and zero errors — "no Google usage found", distinct from unknown.                                                   |

## Split brains (each was a real bug or a real near-miss)

- **Verdicts are derived, never stored** — payloads carry evidence only.
- **Rulesets are provenance, never verdicts** — a stamp says which rules
  made the evidence, not what the domain is.
- **Unknown ≠ dead** — `IsNotFound` conflates NXDOMAIN and NODATA; name
  existence goes through the rcode-level existence oracle.
- **GCP ≠ all Google IPs** — cloud.json only; goog.json would flag
  google.com itself.
- **Suffix matches need label boundaries** — `isSubdomainOf` exists so
  `evil.google.com.evil.example` never matches.
