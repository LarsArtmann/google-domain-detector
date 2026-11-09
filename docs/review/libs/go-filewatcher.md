# go-filewatcher — adoption & utilization review

> Reviewed: 2026-11-09 · Module: `github.com/larsartmann/go-filewatcher/v2`
> Status in google-domain-detector: **NOT IN MODULE GRAPH** (not adopted)
> Audited checkout: `/home/lars/projects/go-filewatcher` @ `9f44dee` = v2.5.0 + 4 docs-only commits

## Verdict

**Do not adopt.** go-filewatcher is a production filesystem-event monitoring
stack (fsnotify + filters + middleware + debouncing + resilience) built for
processes that react to file changes over time. google-domain-detector
reads its input once, scans, renders, and exits — there is no event loop to
feed and nothing to react to.

## What the library is

A composable wrapper around fsnotify (verified in watcher.go, filter.go,
middleware.go, debouncer.go, backend.go):

- Core: `New(paths, opts...)`, `Watch(ctx) (<-chan Event, error)`,
  `WatchOnce`, `Add`/`AddRecursive`/`Remove`/`WatchList`, `Stats`,
  `Errors`, `Reset`, `Close`, `Healthy`.
- Events carry Path/Op/Timestamp/IsDir/Size/ModTime/Hash (event.go:99-118);
  optional SHA-256 content hashing via `WithContentHashing()`.
- ~26 filters with `FilterAnd/Or/Not` composition: extensions, globs,
  regex, size/age bounds, gitignore-aware, generated-code detection.
- 19+ middleware: logging, recovery, throttle/rate-limit, metrics,
  deduplicate, batch, circuit breaker, exponential backoff, OTel.
- Global and per-path debouncing; recursive watching and symlink following
  on by default.
- Resilience: inotify watch-budget handling (ENOSPC degraded mode),
  self-heal, and a polling fallback (`WithPolling`) for NFS/FUSE/Docker.

Dependency weight: fsnotify, gogenfilter/v3, go-gitignore, x/text, x/time
(+11 indirect), and it requires `GOEXPERIMENT=jsonv2` on toolchains ≤ 1.26
(README.md:60-67; the project's Go 1.27 satisfies it by default).

## Current usage in google-domain-detector

None — not imported, not in the graph. The project has no watched
directory, no config hot-reload, and no long-running process.

## Why we should NOT use it

1. **Process-model mismatch.** Every entry point exists to observe live
   changes over time; a one-shot batch scan terminates before any event
   could matter.
2. **Even the hypothetical future feature doesn't justify it.** "Re-run the
   scan when the --input file changes" would be a ~10-line stat/sleep loop
   — or a direct fsnotify use — versus a 5-direct-dep middleware stack with
   an experiment-gated build.
3. Hermetic nix single-binary profile favors minimal deps (STACK.md
   "public deps only, single binary").

## Capability ↔ project applicability

| Capability                     | Applies? | Reason                                              |
| ------------------------------ | -------- | --------------------------------------------------- |
| Watch/WatchOnce event streams  | No       | No long-running phase                                |
| Filters + combinators          | No       | Nothing to filter without events                     |
| Middleware (19+) / debouncing  | No       | Event-pipeline machinery                             |
| Polling fallback / self-heal   | No       | Watch-budget resilience for daemons                  |
| Content hashing events         | No       | Input is read once in full                           |

## Version currency (of the audited checkout)

Tags: v2.5.0 (newest), v2.4.3, v2.4.2, v2.4.1, v2.4.0. HEAD is 4 commits
past v2.5.0, all docs/website. Healthy repo; rejection is about fit, not
quality.

## Revisit trigger

Only if the tool ever becomes a watch-mode daemon (e.g. `scan --watch`
re-scanning as a domain list file grows). That contradicts the one-shot
profile in STACK.md and would warrant a fresh stack decision first.

## How to verify

```bash
rg -n 'filewatcher' --type go . go.mod go.sum    # absent
cd /home/lars/projects/go-filewatcher && git tag --sort=-v:refname | head -5
cd /home/lars/projects/go-filewatcher && rg -n 'GOEXPERIMENT' README.md
```
