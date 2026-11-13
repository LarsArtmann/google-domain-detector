# Dedup baseline — accepted clone groups (2026-11-13)

`art-dupl -t 1` audit iterated to zero harmful duplication. Every clone group
below was reviewed and **accepted**: the similarity is idiomatic or
intentional, and extraction would cost more than it saves. This is the
acceptance record for future dedup audits — a new audit should find exactly
these groups (plus any new code, judged fresh), not a growing list.

## Accepted groups

1. **Append-in-loop collectors** — `internal/cli/replay.go` (comma-split run
   list), `internal/render/render.go` `Filtered` (filter pass),
   `internal/scanstore/supersession.go` `findMissingRuns` (missing-run
   collection). Three packages, three predicates, three element semantics;
   each loop is 3–5 lines. A generic filter helper would take more parameters
   than the duplicated lines.
2. **Dial-and-defer acquire pattern** — `internal/resolve/exists.go`
   `exchangeUDP`/`exchangeTCP` after the `dialDNS` extraction. The shared 6
   lines (`conn, err := dialDNS(...)` / err return / `defer conn.Close()`)
   are the canonical Go resource-acquisition idiom; the `defer` must stay at
   the call site, so any further abstraction (closure callback) trades
   readability for nothing. The functions differ in wire protocol after it.
3. **`return false` negative branches** — `internal/classify/errors.go`
   `IsTransient` (exhaustive enum switch + future-proof default) and
   `internal/domain/domain.go` `validLabel` (label charset rules). Different
   predicates, different packages; the shared token is `return false`.
4. **`ErrorKind` field in two unrelated types** — `internal/classify`
   (DNS lookup-failure kinds: nxdomain/timeout/servfail/canceled) and
   `internal/ranges` (range-feed failure kinds: fetch/corrupt, mapped to
   exit codes). Same field name, disjoint domain contexts; merging would
   couple classify to the feed layer.

## Extractions made this audit

- `resolve.exists.dialDNS` — dial + deadline boilerplate from both exchange
  functions (deadline-bind failure now closes the conn inside the helper).
- `scan.markNXDomain` / `scan.noteLookupError` — the NXDOMAIN stamp and the
  "Err only when the domain yielded zero evidence" invariant (AGENTS.md) in
  one place each.
- `scan.probeRecord[V]` — the TXT/MX/NS probe triplication with the
  NODATA-is-a-fact census rule baked into one generic helper.
- `scan.ScanDomain` — oracle-error and exists paths unified onto a single
  probe tail (explicit `existence = resolve.Exists` on oracle failure).
- `input.Streamer.readFailed` — read-error recording with clean-EOF
  swallowing in one place (three call sites).
- `input.csvHeaderColumn` now delegates to the public `HeaderColumn`
  instead of re-deriving the keyword rule.
- `cli.mapReplayError` — the identical journal-error mapping in
  `runReplayStreaming` and `runReplayBuffered`.
- `render.SupportedFormats` — seen-map double loop replaced by
  `slices.Concat` + `Sort` + `Compact` (same sorted-unique result).

Regression test added: `TestParseLeadingBlankLinesBeforePlainAndCSV` pins the
`detectMode` blank-line skip that the refactor briefly threatened.
