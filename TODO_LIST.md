# TODO List

Short- and mid-term improvement tasks for google-domain-detector. Status
markers: `[ ]` open, `[~]` in progress, `[x]` done.

## Detection coverage

- [ ] Batch retry (once) for transient resolver failures — a flaky local
      resolver currently surfaces as an `Error` cell; one retry would clear
      most of them without hiding hard failures.
- [ ] `--jsonl` streaming output for very large batches (render results as
      they complete instead of after the whole run).
- [ ] Consider CAA/other record classes only if a real fingerprint emerges —
      do not add speculative probes.
- [ ] Optional CSV enrichment mode: emit the input CSV with appended
      workspace/gcp columns, preserving all original columns.

## Operational

- [ ] Release v0.1.0 tag once the first real-world lead list ran clean
      (go-release skill owns the process).
- [ ] Homebrew tap entry (homebrew-tap repo) after the tag.
- [ ] nix run .#release wiring check (go-nix-helpers release app).
