# go-cqrs-lite — adoption & utilization review

> Reviewed: 2026-11-09 · Module: `github.com/larsartmann/go-cqrs-lite` (monorepo, 90+ modules)
> Status in google-domain-detector: **NOT IN MODULE GRAPH** (not adopted)
> Audited checkout: `/home/lars/projects/go-cqrs-lite` @ `7f88d0618` (2026-11-09, 50 commits ahead of newest tag `systemscenario/v4.3.0`)
> Consumer repo state at review: google-domain-detector @06cad49 (2026-11-09) — verdicts are point-in-time; re-verify citations against HEAD before relying on them

## Verdict

**Do not adopt — correct exclusion, keep as is.** The project is a read-only,
stateless, one-shot batch scan; go-cqrs-lite exists to give write models an
auditable event history with optimistic concurrency. There is nothing to
command, decide, persist, project, or replay. STACK.md's recorded exclusion
("no events or invariants — a batch read-only scan (YAGNI)") is validated by
the library's own "when NOT to use this" guidance.

## What the library is

CQRS + event sourcing for Go "without the framework tax": you declare
commands/events/state folds, and the `system.New(ctx, DomainConfig,
DeploymentConfig)` composition root (ADR-0123) wires event store, bus,
projection host, snapshots, health, and lifecycle. `metaengine` is a
cost-based query planner routing each query to the cheapest of 12 storage
engines (memory, sqlite, turso, pebble, bbolt, badger, postgres, mysql,
dgraph, bigtable, duckdb, iroh) with health-driven rerouting. Production
primitives: event signing (HMAC/Ed25519/multisig), payload encryption with
key rotation, upcasters/schema evolution, OTel + Prometheus bridges,
6-family error taxonomy, branded IDs.

Verified against repo README.md and `system/v4/go.mod` (local checkout).

## Current usage in google-domain-detector

None. Not imported anywhere (`rg larsartmann` across the repo shows only
cmdguard, go-output, and internal packages). Not referenced in go.mod, not
even indirectly.

## Why we should NOT use it here

1. **Problem-class mismatch is fundamental.** Event sourcing pays off when
   state changes must be auditable, replayable, and concurrently consistent.
   A scan run produces a rendered report; its only state is one 24 h-TTL
   cache file (`internal/ranges`). No writes exist to journal.
2. **Minimum adoption is heavy.** The prescribed minimum (`system/v4` +
   `metaengine/v4` + one engine blank-import) pulls **24 direct deps** into
   `system/v4` alone — modernc.org/sqlite (an embedded SQL engine),
   Watermill, koanf ×4, an LRU cache lib, test tooling — into a hermetic nix
   single-binary build whose entire current dep tree is tiny by design.
3. **No independently reusable sub-piece.** The adjacent-looking modules
   (`id/v4` branded IDs, error taxonomy, `kv`) duplicate things the project
   already has lighter or deliberately rejected: go-branded-id is already in
   the graph via cmdguard and was explicitly excluded in STACK.md ("scanned
   domains are input values, not entity IDs").
4. **The library itself disqualifies the use case.** Its "when NOT to use"
   section says skip it for apps that are plain CRUD without domain events —
   a fortiori an app with no writes at all.

## Capability ↔ project applicability

| Capability                                  | Applies to this project? | Reason                                                  |
| ------------------------------------------- | ------------------------ | ------------------------------------------------------- |
| Event journal / append semantics            | No                       | No events exist; scan is read-only                      |
| Deciders (load·fold·decide)                 | No                       | No commands, no domain invariants                       |
| Projection host (replay, checkpoints, DLQ)  | No                       | Nothing to project; output is rendered once and dropped |
| metaengine 12-engine query planner          | No                       | No read models; no queries beyond in-memory filtering   |
| Snapshots, scheduling, signing              | No                       | Durability machinery for stateful apps                  |
| cqrs-gen / cqrs-lint / cqrs-upgrade tooling | No                       | Only meaningful inside a cqrs-lite codebase             |

Every row is "No" for a structural reason, not a maturity reason: the repo
itself is very healthy (newest tag 2026-11-09; HEAD actively developed).

## Version currency (of the audited checkout)

Newest tags by date: `systemscenario/v4.3.0`, `system/v4.18.0`,
`metaengine/v4.23.0`, `catalog/v4.14.0`, `metaengine/sqliteengine/v4.8.0`.
HEAD is 50 commits beyond the newest tag. Not pinned anywhere here, so there
is no currency risk for this project.

## Revisit trigger

Adopt only if google-domain-detector ever grows **persistent scan history
with change tracking** — e.g. scheduled re-scans that record per-domain
verdict transitions as an auditable timeline. That would be a different
product (a monitoring service, not a batch CLI), and the STACK.md profile
would need rewriting first.

## How to verify

```bash
rg -n 'cqrs' --type go .                       # no hits outside docs
rg -n 'cqrs' go.mod go.sum                      # absent from module graph
cd /home/lars/projects/go-cqrs-lite && git tag --sort=-creatordate | head -5
cd /home/lars/projects/go-cqrs-lite && git log --oneline systemscenario/v4.3.0..HEAD | wc -l   # ~50
```
