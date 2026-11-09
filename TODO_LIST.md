# TODO List

Short- and mid-term improvement tasks for google-domain-detector. Status
markers: `[ ]` open, `[~]` in progress, `[x]` done.

## Detection coverage

- [ ] Batch retry (once) for transient resolver failures — a flaky local
      resolver currently surfaces as an `Error` cell; one retry would clear
      most of them without hiding hard failures.
- [ ] `--jsonl` streaming output for very large batches (render results as
      they complete instead of after the whole run).
- [ ] Resume/checkpointing for multi-hour runs (`--resume results.jsonl`:
      skip domains already in the output file) — required before any
      whole-zone scan.
- [ ] Consider CAA/other record classes only if a real fingerprint emerges —
      do not add speculative probes.
- [ ] Optional CSV enrichment mode: emit the input CSV with appended
      workspace/gcp columns, preserving all original columns.

## Whole-TLD coverage (the "all active .de/.ch/.uk" goal)

Tranco top-1M covers the traffic-active subset (~63k of the ~30M registered
domains across the three TLDs). Full zone coverage needs the registry data,
which is paperwork only the account holder can sign:

- [ ] .de (~17.5M registered): DENIC RDS (Registered Domain Data Service)
      zone data — member/researcher agreement with DENIC.
- [ ] .uk (~10.5M): Nominet Data Access Agreement (DAA), free but requires
      application + acceptance.
- [ ] .ch (~2.4M): SWITCH zone file under their research/business terms.
- [ ] Pre-filter before scanning at that scale: a cheap single-A-record
      existence pass (or NXDOMAIN pre-sweep) would drop the parked share and
      cut scan volume several-fold.
- [ ] Memory: input currently loads fully into RAM (~30M domains ≈ 1.5 GB) —
      stream input in chunks before any whole-zone run.

## Operational

- [ ] Release v0.1.0 tag once the first real-world lead list ran clean
      (go-release skill owns the process).
- [ ] Homebrew tap entry (homebrew-tap repo) after the tag.
- [ ] nix run .#release wiring check (go-nix-helpers release app).
