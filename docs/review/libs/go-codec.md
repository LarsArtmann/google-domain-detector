# go-codec — adoption & utilization review

> Reviewed: 2026-11-09 · Module: `github.com/larsartmann/go-codec` (single package `codec`)
> Status in google-domain-detector: **NOT IN MODULE GRAPH** (not adopted)
> Audited checkout: `/home/lars/projects/go-codec` @ `7d02310` (`v0.3.3-2-g7d02310`, newest tag v0.3.3)

## Verdict

**Do not adopt.** go-codec is the serialization contract at the center of the
event-sourcing stack (CBOR determinism, COSE signing, codec envelopes). This
project already uses `encoding/json/v2` natively — adopting go-codec's
`JSONCodec` would be an active **regression**: its default build wraps
`encoding/json` v1 (`//go:build !goexperiment.jsonv2`, json_compat_v1.go:1);
the v2-backed variant is opt-in via a GOEXPERIMENT build tag the hermetic
nix build does not set.

## What the library is

A `Codec` abstraction (`Encode/Decode` interface, codec.go:56) over stdlib
JSON and CBOR (fxamacker/cbor/v2), plus raw []byte passthrough. Built for
event-sourced payloads: `DeterministicCodec` (signing guarantee),
`CBORCompactCodec` (RFC 8949 Core Deterministic + unknown-field rejection),
`WrapEncode`/`UnwrapDecode` self-describing envelopes, COSE Sign1/Encrypt0
helpers, format autodetect with a 1 MiB cap, NDJSON/CBOR streaming,
sync.Pool buffer reuse, and per-op metrics hooks. Runtime deps: only
fxamacker/cbor/v2 + go-error-family.

It is NOT a general serialization toolkit: no yaml, no toml, no msgpack, no
compression (verified — zero matches in non-test source).

## Current usage in google-domain-detector

None. The project's serialization is:
- `internal/render/render.go:88-104` — result JSON via `jsontext.NewEncoder`
  + `json.MarshalEncode` (encoding/json/v2), streaming straight to stdout
- `internal/ranges/ranges.go:58-90` — cloud.json feed via one
  `json.Unmarshal`

Both are stdlib, zero added deps, and already on json/v2.

## Why we should NOT use it

1. **It would downgrade the JSON path.** Default `JSONCodec` builds against
   encoding/json v1; this project migrated to json/v2 deliberately
   (STACK.md decision log, 2026-11-09). Swapping in go-codec without a
   GOEXPERIMENT flag reintroduces v1 behavior.
2. **Zero of its differentiators apply.** CBOR determinism, COSE
   signing/encryption, envelope stamps, buffer pools, metrics hooks — none
   solve a problem a one-shot CLI that prints one JSON document and parses
   one ~100 KiB feed has.
3. **Dependency weight for nothing.** Adds fxamacker/cbor/v2 (+ transitive
   x448/float16) and go-error-family as direct deps to a hermetic nix
   single-binary build.
4. Effort: S mechanically — but negative value.

## Capability ↔ project applicability

| Capability                              | Applies? | Reason                                                        |
| --------------------------------------- | -------- | ------------------------------------------------------------- |
| JSONCodec / stdlib JSON wrap            | No       | Project already uses json/v2 directly; default build is v1     |
| CBOR / CBORCompact determinism          | No       | No binary interchange, no signing                             |
| COSE Sign1/Encrypt0                     | No       | No event signing/encryption domain                            |
| Envelope Wrap/Unwrap                    | No       | No self-describing persisted payloads                          |
| AutoDetect / TranscodeToJSON            | No       | Input formats are known (cloud.json, CSV/plain domain lists)    |
| NDJSON/CBOR streaming helpers           | No       | Output is a single JSON doc; no NDJSON I/O                     |
| Pooling / observability / Size diag     | No       | No hot serialization loop                                     |

## Version currency (of the audited checkout)

Tags: v0.3.3 (newest), v0.3.2, v0.3.1, v0.3.0, v0.2.0. HEAD is 2 commits
past v0.3.3. Not pinned here; no currency risk.

## Revisit trigger

Only if the project ever persists binary scan artifacts or signs payloads
(e.g. exporting results as deterministic CBOR for a downstream verifier).
Nothing on the roadmap points there.

## How to verify

```bash
rg -n 'go-codec' --type go . go.mod go.sum    # absent
rg -n 'encoding/json' internal/               # stdlib json/v2 in use
cd /home/lars/projects/go-codec && git tag --sort=-v:refname | head -3
cd /home/lars/projects/go-codec && rg -n 'goexperiment.jsonv2' json_compat_v1.go json_compat_v2.go
```
