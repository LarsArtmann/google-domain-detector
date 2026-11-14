# Code window: runs --format landed, read-model staleness root-caused, M365 fingerprints, lane handoffs

**Session:** 2026-11-13 21:20 → 2026-11-14 01:38 CET (continuation of the
21-15 resume; directive: execute the unblocked TODO rows). **HEAD
b6f79c6, pushed to both remotes.** A sibling session worked the
replay/audit lane concurrently (cqrs-lint follow-ups → `--audit-excluded`
wave names → the file-size split); lane coordination documented in §e.

## a) FULLY DONE

1. **`runs --format` with FULL run IDs** (the poisonfix-debt row) — full
   enum on the runs command (registry-pinned via
   `TestFormatEnumsMatchRegistry`, RunsFlags added); table/markdown keep
   short IDs for humans, every data format (json, jsonl, csv, tsv, yaml,
   toml) carries FULL IDs plus raw run fields (run_id, started_at,
   completed, resumed_from, wave, counts). Four new tests pin the split.
2. **Read-model staleness race root-caused and fixed** (the session's
   core discovery) — `go test` flake investigation turned into a real
   product bug: **folds live in the projection host's workers, not in
   dispatch** (proven by experiments: Boot-recorded journals fold to
   ZERO; `Open` + immediate `ListRuns` deterministically saw zero runs on
   an unfoldled journal). Any read-model query right after `Open` races
   the staggered catch-up drain — stale or EMPTY ledger. This would have
   hit `enrich` (the sales-artifact path) and `runs`/`show` after every
   unclean close. Fix: `scanstore.WaitProjectionsSettled` (bounded 60 s
   poll on worker states, static wrapped errors, exhaustive switch),
   wired into runs/show/enrich; regression test pins the unfoldled-journal
   round trip; recording paths deliberately NOT wired (their workers do
   the folding). `--resume` verified immune (journal walk via Boot).
   AGENTS.md gotcha + post-census library TODO row
   (projectionhost settled-API) recorded.
3. **Data-purity fix found on the way** — the run-count footer leaks into
   csv/tsv output (go-output renders whatever is set); gated to
   table/markdown in BOTH the runs table and the supersession audit
   table, pinned by purity tests.
4. **M365/Exchange fingerprints** (classify-only row) — `mx_microsoft`
   (MX under mail.protection.outlook.com, strong) + `spf_microsoft`
   (include of spf.protection.outlook.com, weak) as `microsoft`-platform
   evidence; both SPF parsers deduplicated into one `spfIncludes`; the
   platform boundary is pinned (Microsoft evidence never upgrades a
   Google verdict — pure-M365 domain detects nothing, mixed domains keep
   their exact Google verdicts); Counts deliberately untouched (journal
   payload schema stays v1 mid-census). FEATURES.md row added.
5. **FormatsForShape (E27) decided** — closed as superseded: T22's
   `render.SupportedFormats()` + enum pin already drift-proofs in both
   directions while keeping the format set an explicit product contract;
   auto-generation would pull in bystander formats from other in-process
   go-output consumers.
6. **TXT truncation row struck** — discovered it was CLOSED with live
   evidence on 2026-11-13 (00-03 report §8: cloudflare.com's 2,539-byte
   TXT parses cleanly; historical failures were resolver distress) but
   the TODO row never got struck; struck with the citation.
7. **describe-battery eyeball** (ErrNoRuns row tail) — all commands and
   flags current; struck with evidence.
8. **All five docs-gates green** after every strike; each unit
   committed and pushed (commit-per-unit with daemon races absorbed per
   the mixed-commit policy; pushed-tree compositions verified via
   throwaway `git worktree` build+test before every push).

## b) PARTIALLY DONE

1. **Wave B attempt 2** — 115,791 scanned at 01:23 (~3.7/s sustained,
   ~38% of the ~304k tail), 0 watchdog alarms all night, runner alive
   8h42m. ETA ~15:00-16:00 today. The poller's 30-min lines lag under
   load (jsonl growth is the live signal); the 01:53 line was pending at
   window close — heartbeat/watchdog cover the wedge case.
2. **Sibling's lanes in flight** (their working tree, deliberately
   untouched): `--audit-excluded` wave names (replay.go refactor — their
   swept snapshots currently fail the lint gate on exactly 2 findings,
   runReplayAudit cyclop 15 + ineffassign isWave) and the file-size
   split (cli_test.go → focused cli_*_test.go files appearing untracked).
3. **CI on the branch** — red on exactly the sibling's 2 mid-flight
   findings; every finding of mine is green. Their session closes it
   when their refactor lands.

## c) NOT STARTED (blocked or next-window)

Post-B census chain (single-flight), file-size split completion (sibling
owns the lane live), io-history-ratchet + TCP contention (post-B data),
rank-band calibration (post-B), zone-ch scan (single-flight), owner rows
(www-strip, GPG, Nominet), asciinema demo, audit-upkeep remainders.

## d) TOTALLY FUCKED UP

**Nothing shipped broken.** Near-misses, owned:
1. Three daemon sweeps bundled my files with sibling content (junk
   messages) — accepted per the mixed-commit policy; no history rewrite
   with a live sibling.
2. One `buildflow format` invocation reformatted beyond my files (the
   sibling's in-flight tree) — content-neutral; their lane re-formats on
   landing.
3. A sloppy import-block edit mangled runs.go mid-flight (caught by
   build, fixed within two edits before any commit).
4. My audit purity test failed against the sibling's MID-FLIGHT swept
   state (their Boot+wait bug) — correctly diagnosed as theirs, not
   fixed by me (lane boundary), resolved by their continued work.

## e) WHAT WE SHOULD IMPROVE

1. **Two live sessions in one repo need explicit lane fences** — this
   window's coordination (I backed off replay/audit the moment the
   sibling's refactor appeared; they extended my audit test rather than
   colliding) worked, but it relied on reading their diffs char-by-char.
   A one-line lane claim in each session's report header would make it
   mechanical.
2. **CI can be red mid-window under concurrent sessions** — the release
   law gates releases, but a "branch green" check before ANY push would
   have deferred my push until the sibling's snapshot was clean. Tradeoff
   accepted: land-per-unit beats green-per-push when the red is provably
   another lane's WIP.
3. **The projection-staleness class is library-shaped** — the repo-local
   wait is correct but every future go-cqrs-lite consumer rediscovers
   this; the TODO row (post-census) should land a `WaitUntilSettled` upstream.
4. **Poller lines lagging under load** is benign but confusing in the
   dark — the watchdog's jsonl-growth signal is the real liveness check;
   consider documenting that hierarchy in docs/census.md.

## f) NEXT (top items, full list in TODO_LIST)

1. Wave B completion ~15:00-16:00 → automatic gate (`--expect-skip
   96215`) → wave C launch + fresh watchdog (single-flight chain).
2. T11-poisonfix-25 → closure audit (0 poisoned) → T15 final
   distribution → closing status report.
3. Verify crt.sh timer run ~04:37-05:00 lands in-repo (plausibility
   guard must accept ≥100k .de rows).
4. Sibling's landing: their wave-names + file-split work → branch green.
5. Library: projectionhost settled-API (post-census).
6. Post-B measurements: rank-band calibration, TCP contention.
7. Then the parked/post-census rows (enrichment join, dedup,
   zone-ch scan, owner decisions).

## g) OPEN QUESTIONS

1. Same as 21-15 §g1 (attended vs auto-chained C launch overnight) —
   defaulting to attended.
2. Same as 21-15 §g3 (www-strip policy) — row stays owner-blocked.
