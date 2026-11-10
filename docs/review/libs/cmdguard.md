# cmdguard — utilization deep dive

> Reviewed: 2026-11-09 · Module: `github.com/larsartmann/cmdguard/v5`
> Status in google-domain-detector: **DIRECT, pinned v5.0.0** (the CLI framework; also the transit route for go-error-family, go-atomic-write, go-branded-id, go-ndjson, samber-do-auditlog, koanf, fang, lipgloss, samber/do)
> Audited checkout: `/home/lars/projects/cmdguard` @ HEAD = `v5.0.0` + 23 commits (only 3 source files changed: `WithStrictPositionalArgs`, `WithArgsPassthrough` — both new opt-ins; mutex-guarded Builder)

## Verdict

**Adoption score: 56/100 (moderate) — 8 fully leveraged + 2 partial of 16
applicable capabilities.** The core path (typed flags, builder, batteries,
error contract, test seam) is exactly right. The gap is a shelf of S-effort
features the single-command CLI never turned on: `validate` tags, doctor
checks, env prefix, exit-code differentiation, recovery middleware, and the
`-o/--output`+JSON-error-envelope plumbing. Version pin is **current**
(v5.0.0 is the newest v5 tag; post-tag additions don't apply here).

## What the library is

Cobra wrapper with typed struct-tag flags (`flag`, `short`, `default`,
`help`, `values`, `required`, `count`, `local`, `hidden`, `validate`, `env`,
`prompt`, `secret`), per-command flag structs, DI scope (samber/do), config
files via koanf (YAML/TOML/JSON, strict keys, `config explain`), batteries
(`version`, `doctor`, hidden `describe` JSON contract), middleware
(Timing/Timeout/Recovery, PostFlagParse, Cleanup), fang-styled help + man
pages + shell completion, structured JSON error envelopes for machine
formats, and an `ExitCoder`/`ExitError` exit-code contract compatible with
go-error-family. Sub-modules: glamour, prompts, spinner, telemetry,
flightrecorder, schema, lint.

## Current usage (baseline, verified)

| Site                        | API used                                                            |
| --------------------------- | ------------------------------------------------------------------- |
| `cmd/.../main.go:18`        | `v5.ExitCode(run(...))`                                             |
| `internal/cli/cli.go:71-76` | `NewCLI(name, desc, RootConfig{}, WithCLIVersion)`                  |
| `internal/cli/cli.go:81-86` | `NewCommand("scan", ScanFlags{}, fn, WithShort(...))`               |
| `internal/cli/cli.go:88-95` | `NewBuilder` + `Try` + `Build`                                      |
| `internal/cli/cli.go:60`    | `ExecuteWithArgs` (repeat-safe, used by tests)                      |
| `internal/cli/cli.go:83`    | `ArgsFromContext(ctx)`                                              |
| `internal/cli/cli.go:36-47` | tags: `flag`,`help`,`short`,`default`,`values`,`env` (6 of 13 tags) |

## Capability inventory & grades

| #  | Capability                                                                                               | Grade              | Evidence / note                                                                                                                          |
| -- | -------------------------------------------------------------------------------------------------------- | ------------------ | ---------------------------------------------------------------------------------------------------------------------------------------- |
| 1  | Typed flag structs + core tag set                                                                        | 🟢 Fully leveraged | cli.go:36-47 — values enum, Duration, env tags all used correctly                                                                        |
| 2  | Builder (NewBuilder/Try/Build)                                                                           | 🟢 Fully leveraged | cli.go:88-95 — error-propagating Try pattern                                                                                             |
| 3  | ExecuteWithArgs + ArgsFromContext + DI seam                                                              | 🟢 Fully leveraged | repeat-safe exec powers cli_test.go; deps injected into runScan                                                                          |
| 4  | version battery (WithCLIVersion)                                                                         | 🟢 Fully leveraged | main.go:15 link-time override + WithCLIVersion                                                                                           |
| 5  | Error contract (print exactly once + ExitCode)                                                           | 🟢 Fully leveraged | main.go:22-23 comment documents the split; no double printing                                                                            |
| 6  | Signal handling / graceful shutdown defaults                                                             | 🟢 Fully leveraged | on by default; ctx cancellation feeds scan.Run                                                                                           |
| 7  | fang styling, man pages, completion                                                                      | 🟢 Fully leveraged | free via defaults (binary already ships `man`, `completion`)                                                                             |
| 8  | describe battery (JSON contract)                                                                         | 🟢 Fully leveraged | auto-registered; documents flags/env for tooling                                                                                         |
| 9  | doctor battery + `WithDoctorCheck`                                                                       | 🟡 Partially used  | doctor exists but with zero real checks — a network tool has an obvious preflight                                                        |
| 10 | Env handling via `WithEnvPrefix`                                                                         | 🟡 Partially used  | env vars work (GDD_RANGES_CACHE/GDD_RESOLVER) but via full-name tags; prefix option would make the GDD_ convention structural            |
| 11 | `validate` tag                                                                                           | 🔴 Missed (S)      | `--workers 0`/negative silently falls back to 8 (scan.go:44) instead of a parse-time error                                               |
| 12 | `ExitError` exit-code differentiation                                                                    | 🔴 Missed (S)      | usage errors (cli.go:178,203) exit 1 — same as DNS/runtime failures; `NewExitError(2, err)` would separate them                          |
| 13 | Middleware (Recovery, Timeout)                                                                           | 🔴 Missed (S)      | a panic in scan aborts with a stack trace instead of a printed error + exit 1                                                            |
| 14 | `WithOutputFormat` + JSON error envelopes                                                                | 🔴 Missed (M)      | `-o json` switches errors to structured envelopes with `hint`/`code` (cli.go:647-657 in cmdguard) — better scripting contract than prose |
| 15 | `WithCLICommit`, `WithExample`, `WithLong`                                                               | 🔴 Missed (S)      | build identification and help examples are one-liners                                                                                    |
| 16 | cmdguard-lint in CI                                                                                      | 🔴 Missed (free)   | consumer passes CG001-CG008 today; a lint step would keep it that way                                                                    |
| —  | Config file (WithConfigFile, config explain)                                                             | ⚪ N/A             | one-shot single-user CLI; flags+env suffice — no config-file audience                                                                    |
| —  | DI scope (Provide/Invoke/Override)                                                                       | ⚪ N/A             | manual injection into runScan is simpler and already the test seam                                                                       |
| —  | Subcommands, groups, hidden/deprecated/prompt/secret tags, prompts/spinner/glamour/telemetry sub-modules | ⚪ N/A             | single command; STACK.md "revisit when command set grows beyond scan" is correct                                                         |

Score inputs: N=19 rows, A=16 applicable, Full=8, Partial=2 →
`round(100 × (8 + 2×0.5) / 16)` = **56 — moderate band**. Impact-weighted
read: the foundation is correctly used; every gap is incremental polish,
and 6 of 8 misses are S-effort.

## Top opportunities (impact × ease)

1. **`validate:"min=1,max=512"` on Workers** (S) — parse-time typo guard;
   removes a silent fallback.
2. **`WithDoctorCheck` preflight** (S-M) — resolver reachability + ranges
   feed fetch as doctor checks; turns a hollow battery into a real
   diagnostic for a network tool.
3. **`NewExitError(2, ...)` for the two usage errors** (S) — scripts can
   distinguish "bad invocation" from "scan failed".
4. **`WithEnvPrefix("GDD_")`** (S) — structural GDD_ convention; future
   flags inherit it; describe output stays accurate.
5. **`RecoveryMiddleware`** (S) — panic → printed error + exit 1.
6. **`WithCLICommit` + `WithExample`** (S) — exact-build version output,
   self-documenting help.
7. **`WithOutputFormat` adoption** (M) — **decided against 2026-11-10**
   (STACK.md decision log): `-o/--output` is a global flag whose resolved
   value is read via the CLI object, conflicting with per-command flag
   structs and the injected-deps test seam; `--format` stays. The registry-
   derived-enum benefit is delivered on `--format` instead (plan T22).
   JSON error envelopes were the other half of this gap and went with it —
   revisit only if scripted consumers ask for machine-readable errors.
8. **cmdguard-lint in CI** (free) — expected zero findings today.

## Misuse check

None found. The Builder/Try pattern, error print-once split, and
repeat-safe ExecuteWithArgs all match the library's documented contract.
cmdguard's own docs understate its tag set (doc.go:65 omits
`values`/`validate`/`local`/`hidden`/`prompt`/`secret`) — library-side doc
gap, not consumer misuse.

## Version currency

Pinned **v5.0.0 = newest v5 tag** (only one). HEAD adds only
`WithStrictPositionalArgs`/`WithArgsPassthrough` (irrelevant: scan wants
free-form domain positionals, i.e. today's passthrough) and a mutex-guarded
Builder (irrelevant: single-goroutine construction). No action.

## How to verify

```bash
go mod graph | rg 'cmdguard'                 # the 5 indirect larsartmann deps all route through it
go run ./cmd/google-domain-detector --help   # styled help, batteries listed
go run ./cmd/google-domain-detector scan --workers 0 --no-ranges example.com   # silent fallback, no validation error (gap #1)
cd /home/lars/projects/cmdguard && git diff --stat v5.0.0..HEAD -- 'pkg/cmdguard/v5' ':!*_test.go'
```
