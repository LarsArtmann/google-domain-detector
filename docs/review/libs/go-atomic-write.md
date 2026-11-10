# go-atomic-write — adoption & utilization review

> Reviewed: 2026-11-09 · Module: `github.com/larsartmann/go-atomic-write`
> Status in google-domain-detector: **INDIRECT v0.6.1** — already compiled into the binary via `cmdguard/v5 → samber-do-auditlog → go-atomic-write` (go.mod:39), but never called directly
> Audited checkout: `/home/lars/projects/go-atomic-write` @ `5688158` = v0.6.2 + 39 commits (only 2 code-relevant: `WriteFuncWithPerm`, MkdirAll-on-demand in `tempPathFor` — both **unreleased**)
> Consumer repo state at review: google-domain-detector @d07a343 (2026-11-09) — verdicts are point-in-time; re-verify citations against HEAD before relying on them

## Verdict

**ADOPT — this is the audit's one clear missed opportunity (effort S).**
The project hand-rolls a weaker version of this exact library in
`internal/ranges/ranges.go:213-228` (writeCache). The library is already in
the build graph; promoting it to a direct dep and replacing the hand-rolled
body costs ~3 lines and gains crash-safe fsync, race-free temp naming, and
error reporting.

## What the library is

Crash-safe, race-free file replacement: stage to a uniquely-named temp file,
fsync it, optionally take an flock + verify an xxhash64 content fingerprint,
then rename and fsync the parent directory (POSIX). Windows path retries
renames on sharing violations. Exactly 12 exported symbols:

| Symbol                                   | What it does                                                              |
| ---------------------------------------- | ------------------------------------------------------------------------- |
| `Write(path, data)`                      | temp + fsync + rename + dir-fsync (atomicwrite.go:17)                     |
| `WriteWithPerm(path, data, perm)`        | explicit perm, temp created with final perm (atomicwrite.go:42)           |
| `WriteVerified(path, data, fp)`          | flock + re-read + fingerprint match required (atomicwrite.go:65)          |
| `WriteIfChanged(path, data) (bool, err)` | xxhash dedup: skips write + mtime bump when identical (atomicwrite.go:91) |
| `WriteFunc{,WithPerm,Verified}`          | streaming via callback through 64 KiB bufio (atomicwrite.go:125/151/167)  |
| `Fingerprint [8]byte` + helpers          | xxhash64 content fingerprint; `FingerprintFile` (fingerprint.go:13-35)    |
| `ErrConcurrentModification`              | sentinel for verified-write races (commit.go:13)                          |

fsync verified both places: temp file at stage.go:88/119, parent dir after
rename at rename_unix.go:17-34. Locking exists only on `*Verified` paths
(commit.go:36-44); plain writes rely on unique temp names. Default perms:
preserve existing file mode else 0o644 (stage.go:16-31). No TTL/mtime
helpers — the caller's mtime-TTL logic stays hand-rolled either way.

## Current usage in google-domain-detector

None directly. The hand-rolled code it should replace:

```go
// internal/ranges/ranges.go:213-228
func writeCache(path string, data []byte) {
    if path == "" { return }
    if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil { return }
    tmp := path + ".tmp"
    if err := os.WriteFile(tmp, []byte(data), 0o600); err != nil { return }
    _ = os.Rename(tmp, path)
}
```

Weaknesses vs the library: no fsync of file or dir (a crash mid-write can
leave a torn cache — tolerated today only because readCache falls back to
the network); fixed `.tmp` name (two concurrent runs can interleave writes
to the same temp path); errors silently swallowed; no temp cleanup on
failure.

## Recommendation (concrete)

```go
import atomicwrite "github.com/larsartmann/go-atomic-write"

func writeCache(path string, data []byte) {
    if path == "" { return }
    if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil { return }
    _ = atomicwrite.WriteWithPerm(path, data, 0o600) // best-effort cache
}
```

On released **v0.6.1/v0.6.2**, keep the caller-side `MkdirAll` (library-side
MkdirAll is unreleased, and its hardcoded 0o755 would _regress_ today's
0o700 cache dir).

**Caution — do NOT use `WriteIfChanged` here:** the cache TTL is mtime-based
(ranges.go:195-197). Skipping an identical-content write means the mtime
never refreshes, the 24 h TTL never restarts, and every post-TTL run
re-fetches the feed. `WriteWithPerm` is the right call.
`WriteVerified`/`ErrConcurrentModification` are also anti-fits: last-writer-
wins is correct semantics for a cache.

## Why it is worth it (and the cost)

- Gains: durability on crash, race-free concurrent runs, temp cleanup,
  wrappable errors — for zero new binary weight (already linked in).
- Cost: one direct-require promotion (`go get` + tidy), 3-line swap, one
  test tweak. Effort **S**; risk functionally nil on released tags.

## Version currency

Pinned: v0.6.1 (indirect). Latest release: **v0.6.2**. Local HEAD is
v0.6.2 + 39 commits, but 37 are website/branding/docs/CI; the only two
unreleased code changes are `WriteFuncWithPerm` and MkdirAll-on-demand.
Recommendation targets released v0.6.2; bump when adopting.

## How to verify

```bash
go mod why github.com/larsartmann/go-atomic-write   # compiled via cmdguard→auditlog
rg -n 'os.Rename|WriteFile' internal/ranges/ranges.go   # the hand-rolled path today
cd /home/lars/projects/go-atomic-write && git tag --sort=-v:refname | head -3
cd /home/lars/projects/go-atomic-write && git log --oneline v0.6.2..HEAD -- '*.go' | grep -v website
```
