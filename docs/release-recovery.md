# Release-note emergency runbook (rehearsed 2026-11-13)

Two different emergencies, two different procedures. The immutable thing
is the TAG + module content (proxy + checksum log); almost everything
visible to humans is mutable. Know which one is on fire.

## A: The release NOTES are wrong (docs-only emergency)

The GitHub Release body is editable; the tag and proxy content are not.
This is a pure documentation fix — no code, no re-tag, no re-release.

Rehearsed path (proven against v0.1.0 as a byte-identical no-op):

```bash
# 1. Fetch the CURRENT body exactly (raw API; --jq appends a newline —
#    compare through the raw JSON, never through jq output).
gh api repos/LarsArtmann/google-domain-detector/releases/tags/v0.1.0 > /tmp/rel.json
python3 -c 'import json,sys; sys.stdout.write(json.load(open("/tmp/rel.json"))["body"])' > /tmp/notes.md

# 2. Fix /tmp/notes.md (or write the corrected body from scratch).

# 3. Publish the corrected body.
gh release edit v0.1.0 --notes-file /tmp/notes.md

# 4. Verify the stored body is exactly what you intended.
gh api repos/LarsArtmann/google-domain-detector/releases/tags/v0.1.0 > /tmp/rel2.json
python3 -c 'import json; print(len(json.load(open("/tmp/rel2.json"))["body"]))'
```

Gotchas learned in the rehearsal:

- `gh release view --json body --jq .body` output is NOT the stored body
  byte-for-byte (jq appends a newline). Diffing jq output before/after an
  edit produces phantom one-byte changes; parse the raw JSON instead.
- `gh release edit --notes-file` succeeds even for a no-op edit — always
  re-fetch and verify rather than trusting the exit code.

## B: The released CONTENT is broken (module retraction)

When the tagged code itself is the problem (bad go.mod, broken build,
leaked replace directive), the fix is a NEW version plus a retraction —
never delete-and-retag (proxy + sumdb serve the original forever; see
the go-release skill, failure-modes R1/R2).

Mechanics (rehearsed on a throwaway go.mod copy — NEVER run the retract
edit against the real go.mod outside an actual emergency):

```bash
cp go.mod /tmp/rehearsal.mod
go mod edit -retract=v0.1.0 /tmp/rehearsal.mod   # appends: retract v0.1.0
git diff --no-index go.mod /tmp/rehearsal.mod    # inspect, then discard
```

Real path, in order:

1. Fix the defect on master; land it.
2. Cut the fixed version through the normal release process (go-release
   skill), with `retract v0.1.0 // <reason> — use v0.1.1` in the new
   go.mod (`go mod edit -retract=v0.1.0` on the REAL go.mod this time).
3. CHANGELOG row documenting the retraction.
4. Edit the broken release's NOTES (procedure A) to point at the fixed
   version — the retraction directive only helps `go get` users who read
   go.mod; humans read the release page.
