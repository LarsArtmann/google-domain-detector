# Owner-decision package: everything blocked on you, one page

**Date:** 2026-11-14 02:18 CET · **Status at writing:** wave B running
(122k+ scanned, 0 alarms), tree clean, CI green pending the pushed lint
fix, census chain sequenced behind single-flight.

Each decision below blocks exactly one lane; nothing else waits on them.
Answers 1–3 gate wave C, which can launch any time after B's gate PASS
(~15:00 today). Reply by number is enough.

| # | Decision                                   | Recommendation                        | Blocks                    |
| - | ------------------------------------------ | ------------------------------------- | ------------------------- |
| 1 | Wave-C launch mode: attended vs auto-chain | Attended (status quo)                 | Wave C                    |
| 2 | RulesetV2 stamp before wave C              | YES (see decision note)               | Wave C                    |
| 3 | poisonfix-25 launch after C                | YES, as planned                       | Closure audit             |
| 4 | www-strip policy for 26 wave-A inputs      | Normalize to bare suffix, rescan in C | 26 domains' verdicts      |
| 5 | GPG signing for agent commits              | Decide once, apply everywhere         | All future commits        |
| 6 | BuildFlow#56 fleet rebuild approval        | Approve (one-time host fix)           | Pre-commit hook usability |
| 7 | Nominet .uk email send                     | Send (draft ready)                    | Whole-.uk coverage        |
| 8 | M365 surface + DKIM probe                  | Post-census, render-side tally        | Nothing (parked)          |

## 1. Wave-C launch mode (open since 21-15 §g1)

**Question:** when wave B gates PASS (~15:00–16:00 today), do I launch
wave C unattended (auto-chain), or do you want the handoff attended?

- **Attended (recommendation):** B's gate result lands, C launches on
  your go (or a session you're present in). Matches every report's
  default so far; unattended failure handling stays unexercised.
- **Auto-chain:** `census-wave.sh` already embeds gate + cleanup +
  snapshot; the chain would re-run it for `tranco1m-c` on PASS with a
  fresh watchdog. Risk: a mid-night failure (TCP-upstream outage class)
  burns hours before anything notices; the watchdog polls but does not
  relaunch.

## 2. RulesetV2 stamp (blocking, time-sensitive)

Full analysis + execution plan:
`docs/review/2026-11-14_02-18-ruleset-v2-m365-provenance-decision.md`.
Short version: the ruleset contract's own MUST ("new evidence kinds") is
violated by running wave C under V1 after `5fab996`; the bump is two
lines, migrates nothing (append-only stamps), and re-arms the
cross-ruleset delta guard for the B→C comparison. Google-side numbers
stay comparable across all waves either way (test-pinned invariance).

## 3. poisonfix-25 launch

Input `reports/census/waves/tranco1m-a-poisonfix.txt` (25 audit-derived
domains), fresh binary, after wave C frees the single-flight. Closure
proof is mechanical: `replay --audit-excluded <4 run IDs>` → 0 poisoned.
I hold this as the last census write; say go (can ride with #1's
answer).

## 4. www-strip rescan slot (policy already decided)

The POLICY is closed — docs/census.md records the 2026-11-12 decision:
www-prefixed inputs are normalized to the bare registrable suffix and
scanned. The only open piece is execution: wave A recorded 26 such
inputs under their literal form; rescanning them under the normalized
form inside wave C's pass costs ≈ zero (26 domains) and aligns the
journal with the decided policy. Recommendation: fold the rescan into
wave C. Say go / leave it.

## 5. GPG signing policy for agent commits

Agent commits (mine and siblings') are currently unsigned. If you want
them signed, the clean shape is: a dedicated signing subkey, `gpg`
agent config on this host, and git `user.signingkey` per repo (or
global). Until decided, nothing changes — the daemon and sessions keep
committing unsigned. Say "sign with <key>" or "keep unsigned".

## 6. BuildFlow#56 fleet rebuild (approval gate)

The installed buildflow binary (built with go1.27.0) fails valid commits
on this repo: its embedded analyzers demand a `go mod tidy` the repo's
own toolchain proves is a no-op (go 1.27.2 floor). The fix is rebuilding
buildflow with the current toolchain — a host-level tool change, hence
your approval. Until then the pre-commit hook stays bypassed
(`--no-verify` + direct gate runs, the documented workaround). Approve
and I rebuild it in the buildflow repo; ~10 min.

## 7. Nominet .uk email

Draft ready for your review: `docs/zone-data/nominet-uk.md`. Sending +
the MiTek ID verification are human actions. This unblocks whole-.uk
coverage (~10.5M domains), the largest remaining single block of the
whole-TLD goal.

## 8. M365 surface + DKIM microsoft selector probe

Recommendation (details in the §2 note): park both post-census. Evidence
already renders end-to-end; the missing pieces are aggregation and a
verdict-shaped accessor, both product decisions best made with the
census numbers in hand.
