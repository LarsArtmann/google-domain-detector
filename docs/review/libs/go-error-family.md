# go-error-family — adoption & utilization review

> Reviewed: 2026-11-09 · Module: `github.com/larsartmann/go-error-family`
> Status in google-domain-detector: **INDIRECT v0.12.0** — compiled into the binary via `cmdguard/v5 → samber-do-auditlog → go-error-family` (go.mod:40); never called directly
> Audited checkout: `/home/lars/projects/go-error-family` @ `7ccca5c` = v0.12.0 + 6 commits, **zero .go changes** (README/metadata only) — API-identical to the pin

## Verdict

**Optional scoped adopt (S), not required.** The only place with real
semantic value is `internal/ranges`: network fetch failures (Transient) vs
corrupt-feed parse failures (Corruption) map onto distinct families, and
cmdguard already honors `family.ExitCoder` (cli_errors_json.go:39-52 in
cmdguard), so a scoped adopt would yield machine-distinguishable exit codes
(75 EX_TEMPFAIL vs 65 EX_DATAERR) almost for free. Everywhere else —
especially the DNS path — adoption is **not** recommended: the default
`Transient` classification is fail-open (classifiers that match nothing
return true, classification.go:48-61), so NXDOMAIN-style permanent DNS
failures would silently classify as retryable if any retry machinery ever
consumed them.

## What the library is

Error families + classification + exit codes, verified in the checkout:

- **Families** (family.go:30-50): `Rejection` (bad input, unauthorized, not
  found), `Conflict`, `Transient`, `Corruption`, `Infrastructure` (cannot
  serve), `Orchestration` (internal coordination failure).
- **Constructors** (constructors.go:10-97+): `New`/`Newf`/`Wrap`/`Wrapf` plus
  per-family convenience (`NewTransient`, `WrapConflict`, ...) with stable
  machine `code` strings; `Code(err)` extracts them from any chain
  (classify.go:36-44).
- **Classification** (classify.go:46-69): `Classify(err)` checks the
  `Classified` interface → `Retryable` inference → registered sentinels →
  registered `Classifier` predicates (for dynamic third-party errors like
  sqlite) → **default Transient (fail-open for retry)**. `IsRetryable`, and
  a scoped `Registry`/`NewRegistry` for test isolation.
- **Exit codes** (classify.go:71+): `ExitCode(err)` honors an `ExitCoder`
  chain override; cmdguard maps family → exit code (and its JSON error
  envelope carries `hint`/`code` fields — cmdguard's own errors_facts.go,
  not this library).
- Also: `http.go` HTTP-status helpers, `log.go`, `handle.go`.

## Current usage in google-domain-detector

None directly. Current error style is plain `fmt.Errorf`/`errors.New`
throughout (e.g. ranges.go:61,71,78,86,180,187; cli.go:178,203) with
`%w` wrapping; scan results carry string errors, not error values.

## Should we use it?

**For (scoped to internal/ranges):**

- `ranges.Load` has exactly two error classes the OS already distinguishes:
  fetch (temporary, exit 75) vs parse (data, exit 65). Today both exit 1
  via generic wrapping.
- Zero binary cost — the package is already linked via cmdguard.
- Improves the warning message printed at cli.go:130 with an actionable
  hint ("cached ranges unavailable, continuing without IP evidence").

**Against (default/global):**

- Fail-open Transient default is a footgun for DNS errors — the scan path's
  errors are deliberately plain strings in results (`classify.Result.Err`).
- The project has no consumer of classifications (no retry, no HTTP
  mapping), so most value would be inert.
- STACK.md profile favors stdlib-shaped simplicity for a single-user tool.

Net: adopt only if differentiated exit codes / machine-readable failure
classes for the ranges feed are wanted; otherwise leaving it indirect is
defensible.

## Capability ↔ project applicability

| Capability                 | Applies?              | Reason                                                                                    |
| -------------------------- | --------------------- | ----------------------------------------------------------------------------------------- |
| Family classification      | Partially             | Only ranges fetch-vs-parse split is real                                                  |
| ExitCode via cmdguard      | Yes (if scoped adopt) | free exit-code differentiation                                                            |
| `code` strings + Code(err) | Partially             | pairs with cmdguard's JSON error `code` field (envelope plumbing unused, see cmdguard.md) |
| Classifier registry        | No                    | no dynamic third-party error types in the scan path                                       |

## Version currency

Pinned v0.12.0 (indirect) = newest tag; HEAD adds no code. Fully current.

## How to verify

```bash
go mod why github.com/larsartmann/go-error-family   # via cmdguard→samber-do-auditlog
rg -n 'error-family' --type go .                     # no direct usage today
cd /home/lars/projects/go-error-family && rg -n 'Default — Transient' classify.go
cd /home/lars/projects/cmdguard && rg -n 'ExitCoder' pkg/cmdguard/v5/cli_errors_json.go
```
