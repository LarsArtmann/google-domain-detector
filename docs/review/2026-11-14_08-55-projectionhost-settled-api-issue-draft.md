# projectionhost: ship a settled / catch-up API (upstream issue draft)

**Status:** DRAFT — ready to file against `github.com/larsartmann/go-cqrs-lite`
(repo: go-cqrs-lite), component `projectionhost/v4` (pin at writing:
v4.7.1). Not filed: upstream communication is owner-gated.
**Reference implementation:** this repo's interim
`internal/scanstore/projections.go` (`WaitProjectionsSettled`), pinned by
`TestWaitProjectionsSettledMakesUnfoldedJournalVisible`.

## Title

`projectionhost: add WaitUntilSettled (or equivalent) so consumers can read their writes after Open`

## Body (ready to file)

### Problem

Projection folds live in the host's projection **workers**, not in event
dispatch. A consumer that opens a store whose journal tail was never
folded — unclean close, a writer that bypassed the host (our Boot-shaped
journal walk), or a fresh environment replaying a snapshot — gets a
**deterministically stale or EMPTY read model** from `Open` + immediate
query. We reproduced this 100% of the time: `Open` followed by an
immediate `ListRuns` on an unfoldled journal saw **zero runs** until the
workers' staggered catch-up drain happened to finish. There is no API to
wait for that drain.

### What the API already offers, and why it is not enough

| Existing API                       | Why it does not answer "am I caught up?"                                                            |
| ---------------------------------- | --------------------------------------------------------------------------------------------------- |
| `Status() []WorkerState`           | Exposes states, but the consumer must poll and interpret them (the loop below).                      |
| `CheckStaleness(maxStaleness)`     | Wall-clock staleness since last processed — an idle-but-behind worker can look fine; not position-based. |
| `LagDuration()` / `LagPerProjection()` | Time since the worker's most recent event, not distance to the journal head.                      |
| `RegisterAndWait`                  | Covers registration-time drain only; does not help an `Open` on an already-registered host.          |

### What consumers hand-roll today (the loop every consumer rewrites)

```go
const settlePoll = 25 * time.Millisecond

for {
    states := sys.WorkerStatus()
    settled := true
    for _, w := range states {
        switch w.Status {
        case WorkerLive, WorkerStopped:
            // settled
        case WorkerFailed:
            return w.LastError
        default: // Idle, Running, Backoff, Draining
            settled = false
        }
    }
    if settled || ctx Done / budget exceeded { return }
    sleep(settlePoll)
}
```

Census-critical read commands in `google-domain-detector` (runs, show,
enrich) must call this after `Open`, before the first read-model query;
recording paths must not (their workers do the folding).

### Proposal

A position-based, blocking API on `Host` (system composition root
exposure welcome):

- `WaitUntilSettled(ctx) error` — blocks until every worker is
  `WorkerLive` (tailing live) or `WorkerStopped` (drained to head and
  exited); fails fast on `WorkerFailed` (restarts + last error); honors
  ctx cancellation; optional per-call budget option.
- Implementation can be state-polling initially (the loop above), but a
  checkpoint/journal-head comparison would remove the poll interval and
  make it exact.
- Consider also returning the per-worker end states so callers can
  log/summarize the drain.

### Why the library should own it

The host already owns staggered start, restart/backoff, and graceful
drain — "the last loop every consumer rewrites." Catch-up-before-read is
the same class of loop, and getting it wrong is silent data staleness,
not a crash. Our pinned regression test
(`TestWaitProjectionsSettledMakesUnfoldedJournalVisible`) shows the
failure mode; the API would let consumers delete the hand-roll.

### Environment

- go-cqrs-lite `projectionhost/v4` v4.7.1, `system/v4` v4.18.0,
  `metaengine/v4` v4.23.0 (Map/sqlite projections).
- Reproduced on sqlite journals with a journal tail recorded without a
  running host; consistent across memory and sqlite engines (same
  fold-dispatch plan).

## Filing checklist (verify-before-filing)

- [x] Gap verified at the pinned version (`go doc -all projectionhost/v4`:
      no settled/catch-up method; `CheckStaleness`/`LagDuration` are
      time-based; `RegisterAndWait` registration-only).
- [x] Failure mode proven by experiment + pinned test in this repo.
- [x] Hand-roll cited verbatim from `internal/scanstore/projections.go`.
- [ ] Owner go to actually file (external communication).
