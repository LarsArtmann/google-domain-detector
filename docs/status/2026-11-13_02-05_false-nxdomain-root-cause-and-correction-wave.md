# Status Report — False-NXDOMAIN root cause found and fixed; tranco1m-a-nxfix correction wave running

**Written:** 2026-11-13 02:05 CET · **Session:** continuation of the
census lane (post 01-04 report, owner instruction: execute until done) ·
HEAD `3af80f9` + authored fix commit `7b45a1b` · binary
`reports/census/unbound/gdd-20261113` (sha256 `13dd192d05b0…`,
vcs.revision 7b45a1b, vcs.modified=false).

## The finding (one paragraph)

Every prior session's "false/lying NXDOMAIN" observation — the 01-04
report's burst theory, the E10/pilot forensics, the patch cleanup
residual — had ONE root cause, and it was never the resolver: **Go's
net.Resolver maps NXDOMAIN (name does not exist) AND NODATA (name exists,
no such record) into the same `DNSError{IsNotFound:true}`**, and
`scan.ScanDomain` treated any sub-lookup IsNotFound as an NXDOMAIN
verdict. Every live domain with no MX record and no Google-relevant
evidence (no SPF/verification/Cloud-DNS/Google-CNAME) was stamped
`error_kind=nxdomain` — deterministically (worker-count independent),
and permanently, since NXDOMAIN is domain truth and is never retried by
cleanup. Quantification: **22,010 of wave A's 97,265 domains (22.6%)
carry NX verdicts; a 30-domain sample found 27 alive at Quad9 (~90%
false)** — windowsupdate.com, aliexpress.us, pp.ua, linodeobjects.com
among them. The falsifying experiment chain: patch c4 produced 6 NX
verdicts for domains that answered NOERROR via direct Quad9 AND via the
same forwarder in isolation minutes before and after; they reproduced
IDENTICALLY through a fresh-cache TCP-upstream instance (ruling out
resolver path, cache, and load); a pure-stdlib probe showed
`LookupMX("kwai.net")` → IsNotFound while TXT succeeded (3 records); the
error string names the MX lookup. The earlier theory in the 01-04 report
("burst-window forwarding returns wrong authoritative answers",
"unbound UDP-port starvation fabricates negatives") is **falsified** and
annotated as such in that report and in docs/census.md.

## The fix (committed, tested, lint-zero)

- `resolve.LookupExistence(ctx, host) (Existence, error)` — minimal DNS
  wire client (`internal/resolve/exists.go`): A query, header-rcode
  verdict (NOERROR→Exists incl. NODATA; NXDOMAIN→NotExists;
  SERVFAIL/refusal→temporary DNSError), TC→TCP retry, ctx-deadline
  bounded, resolv.conf fallback only for the no-`--resolver` mode.
- `scan.ScanDomain` rewired: oracle FIRST (NotExists = true NXDOMAIN,
  short-circuit, now actually true); below Exists, sub-lookup IsNotFound
  = ordinary record absence (`isRecordAbsence`) — no error, no evidence,
  no false stamp. Oracle errors are transient-class and probe-on (a
  partial failure with evidence stays a normal result, per AGENTS.md).
- Detection rules UNCHANGED — ruleset stays `v1-2026-11-10`; runs are
  distinguished by wave id + binary provenance, not a ruleset bump.
- Verified: full `go test ./...` 9/9 green (new tests:
  oracle verdict table incl. TC→TCP against a dual-listener test server,
  `TestScanDomainNoMXIsNotNX` pinning the kwai.net shape, NX-no-retry
  rewritten against the oracle); `golangci-lint run` → **0 issues**
  repo-wide; live end-to-end: all 6 former false-NX specimens now scan
  clean, a genuinely dead name still verdicts nxdomain; post-fix NX
  verdicts in the running wave are **12/12 confirmed NXDOMAIN at
  Quad9**.
- Two incidental traps found and documented (AGENTS.md Gotchas):
  DNS-over-TCP cannot be forced via Go's Dial callback (framing follows
  the network string Go passed in); DNSError.Server displays the
  resolv.conf address even when the scan's dial was rewritten to
  `--resolver` — the ":53" error strings that once caused a
  contamination panic are a formatting artifact, not evidence.

## Executed this session (chronological)

1. Inherited the two sibling reports; adopted their gating conditions
   (patch residual 25/2,731 = 0.92% over bar; `gdd doctor` pre-flight
   law; census-wave.sh hardened runner with fresh-pass policy).
2. Pre-flight green (single-flight clear, upstreams NOERROR, forwarder
   NOERROR, `gdd doctor` 4/4). Patch cleanup **round 4** (manual, w2/12s,
   wave `tranco1m-a-patch-c4`, journal run `01M794AXA8YVQC4RNZD8000001`):
   10 clean, 6 NX, 9 unknown → the 6 NX cross-checked FALSE at Quad9 →
   red flag → root-cause hunt (above).
3. Resolver infrastructure: extended `scripts/unbound.sh` with
   `GDD_UNBOUND_CONF` per-instance lifecycle (pid/log derive from the
   conf basename; status health probe now `+tcp`, valid for both
   instances); promoted the TCP-upstream config to tracked
   `scripts/unbound-tcp.conf` (`tcp-upstream: yes` — the key option the
   earlier draft missed: `do-udp:no` alone only changes the client hop).
   Started it on 5354; `gdd doctor` 4/4 through it.
4. Patch correction round **c5** through the TCP-upstream instance
   (15 suspect domains): same 6 NX reproduced → resolver-path theory
   falsified → scanner-side hunt → stdlib MX probe → **root cause**.
5. The fix (commit `7b45a1b`): existence oracle + NODATA-is-absence,
   tests, lint-zero. Built `gdd-20261113` from the clean tree;
   PROVENANCE.md entry (supersedes gdd-20261112 for all future waves).
6. **Launched `tranco1m-a-nxfix`** (22,136 domains = wave A's 22,010 NX +
   the patch wave's 126 NX, disjoint; manifest sha256
   `472fffefcdd2…`) detached via census-wave.sh, w4/10s,
   `--resolver 127.0.0.1:5354`, `GDD_PREFLIGHT_TRANSPORT=tcp` (sibling's
   preflight-transport addition). Pre-flight all green (TCP probes).
   Early numbers: 97.3% clean, 1.9% NX (true deaths), 0.9% timeout at
   1,018 scanned; ~2.4 dom/s → ETA ≈ 3 h. The wrapper's cleanup loop +
   coverage gate + snapshot apply as for any wave.
7. Docs truth pass: census.md section rewritten to the true root cause
   (falsification chain inline), NXDOMAIN error-semantics refinement,
   wave-ledger rows (patch correction + nxfix), AGENTS.md gotchas,
   TODO_LIST closures (nxdomain-bucket row, T11-patch), 01-04 report
   annotated as falsified. Commits `7b45a1b`, `3af80f9` (authored;
   earlier intermediate states were daemon-swept).

## What this unblocks (wave B/C gating)

- The scanner bug would have poisoned waves B/C at ~20% NX-classified
  (of which ~90% false): the fix was a **launch gate**, not optional
  polish. B now additionally requires: nxfix wave clean + a 5k pilot
  PASS through the TCP-upstream forwarder (throughput measurement for
  the TCP hop) + the standing owner go.
- The `--verify-nxdomain N` gate mode (01-04 §f7) is superseded by the
  oracle: post-fix NX verdicts are rcode-authoritative; the
  quad9-cross-check is only needed when auditing runs made with
  PRE-fix binaries (all covered by nxfix + the journal's run ids).

## NOT done / held

- Wave B/C: gated as above (owner go standing).
- T15 census report: after waves complete; must account for nxfix
  last-wins supersession + pin excluded runs.
- The 3 remaining stable-SERVFAIL/timeout residuals from patch c4
  (edgecastdns.net, hofgarten-heiligenhafen.de, patria.org.ve class):
  legitimate E10 candidates post-nxfix, if they persist through the
  wave's own cleanup rounds.
- Push to remotes: not done this session (3+ commits ahead; push with
  the next natural lane close per repo habit).

<!-- fresh:skip sibling-scoped: enrich/stats lanes, docs-gates audit -->
