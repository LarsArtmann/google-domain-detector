# go-credentials-store — adoption & utilization review

> Reviewed: 2026-11-09 · Module: `github.com/larsartmann/go-credentials-store`
> Status in google-domain-detector: **NOT IN MODULE GRAPH** (not adopted)
> Audited checkout: `/home/lars/projects/go-credentials-store` @ `f8855a2` = v0.3.1 + 8 chore commits
> Consumer repo state at review: google-domain-detector @f5ea3ac (2026-11-09) — verdicts are point-in-time; re-verify citations against HEAD before relying on them

## Verdict

**Do not adopt.** go-credentials-store provides multi-backend secret
storage (OS keyring via zalando/go-keyring: Secret Service, macOS
Keychain, Windows Credential Manager; encrypted file store with argon2id
AES-GCM; in-memory store) with per-field lookup, migration, rotation
history, and required-field boot validation. google-domain-detector needs
**zero credentials by design**: it queries public DNS and one public CDN
feed, deliberately has no API mode, and its STACK.md profile demands
public deps only. Introducing a secrets store would create a secret to
store.

## What the library is

Verified in the checkout: `Store` interface (`Ping`, `Get`, `Set`,
`Fields`, `Delete`, `Close`), file-store `Options` (argon2id via
golang.org/x/crypto, configurable salt/nonce sizes, keyring mode FILE,
optional keyring fallback `FILE_PREFER_KEYRING`), multi-field structs
(credentials.go), migration runner + JSON backup/restore (migration.go),
rotation history retention, boot validation of required credentials,
lookup strategies (EXACT → LABEL → ANY, credential lookup precedence),
examples for OpenAI/Anthropic-style API keys.

## Current usage in google-domain-detector

None — not imported, not in the graph.

## Why we should NOT use it

1. **No credentials exist.** Public DNS + public feed; auth-free by
   product definition (README: no API keys needed).
2. Even hypothetical future Google-API enrichment would belong behind a
   deliberate stack decision (STACK.md), not snuck in via a secrets store.
3. Keyring backends would drag OS integration + crypto deps into the
   hermetic nix single-binary build.

## Capability ↔ project applicability

| Capability                 | Applies? | Reason              |
| -------------------------- | -------- | ------------------- |
| Keyring/file/memory stores | No       | No secrets to store |
| Field structs + lookup     | No       | No credential model |
| Migration / rotation       | No       | Nothing to migrate  |
| Boot validation            | No       | Nothing to validate |

## Version currency (of the audited checkout)

Tags: v0.3.1, v0.3.0, v0.2.1, v0.2.0, v0.1.0 (HEAD = v0.3.1 + chores;
one tag-date anomaly: v0.2.1 tagged later than v0.3.1's date but lower
version). Healthy; rejection is fit.

## Revisit trigger

Only if an authenticated data source is ever added (e.g. Google Workspace
Admin SDK verification). That would be a product pivot requiring a fresh
STACK.md decision.

## How to verify

```bash
rg -n 'credentials-store|keyring' --type go . go.mod go.sum    # absent
cd /home/lars/projects/go-credentials-store && git tag --sort=-v:refname | head -5
cd /home/lars/projects/go-credentials-store && rg -n 'type Store interface' *.go
```
