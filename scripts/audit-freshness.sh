#!/usr/bin/env bash
# Audit-freshness guard (E31.4): the library audits under
# docs/review/libs/ are pinned to the dependency versions they reviewed
# ("Status ... pinned vX.Y.Z") and to a consumer commit ("Consumer repo
# state at review @sha"). This guard re-parses those pins and compares
# them against go.mod: a module whose go.mod version no longer matches
# the audited pin has drifted, and the audit's verdicts are stale until
# re-verified.
#
# Exit 0 = every pinned audit is current; exit 1 = at least one stale
# (each named); exit 2 = usage/parse trouble.
#
# usage: audit-freshness.sh [repo-root]
#   repo-root defaults to the script's repo
set -euo pipefail

root=${1:-$(cd "$(dirname "$0")/.." && pwd)}
libs_dir=$root/docs/review/libs
go_mod=$root/go.mod

if [ ! -d "$libs_dir" ] || [ ! -f "$go_mod" ]; then
	echo "audit-freshness: expected docs/review/libs/ and go.mod under: $root" >&2
	exit 2
fi

# go.mod require lines, block and single-line form:
#   <tab>path vX.Y.Z[ // indirect]
#   require path vX.Y.Z
mod_version() {
	awk -v want="$1" '
        $1 == "require" && $2 == want { print $3; exit }
        $1 == "require" && $2 == "(" { inblock = 1; next }
        inblock && $1 == ")" { inblock = 0; next }
        inblock && $1 == want { print $2; exit }
    ' "$go_mod"
}

stale=0
checked=0
printf '%-28s %-10s %-10s %s\n' 'AUDIT' 'PINNED' 'GO.MOD' 'VERDICT'
for audit in "$libs_dir"/*.md; do
	name=$(basename "$audit" .md)

	module=$(sed -n 's/^>.*Module: `\([^`]*\)`.*/\1/p' "$audit" | head -1)
	pinned=$(sed -n 's/.*pinned \(v[0-9][^ *]*\).*/\1/p' "$audit" | head -1)

	if [ -z "$module" ]; then
		printf '%-28s %-10s %-10s %s\n' "$name" '-' '-' 'SKIP (no Module header)'
		continue
	fi
	if [ -z "$pinned" ]; then
		printf '%-28s %-10s %-10s %s\n' "$name" '-' '-' 'SKIP (no version pin; prospective or non-dependency)'
		continue
	fi

	current=$(mod_version "$module" || true)
	if [ -z "$current" ]; then
		printf '%-28s %-10s %-10s %s\n' "$name" "$pinned" '-' 'SKIP (module not in go.mod)'
		continue
	fi

	checked=$((checked + 1))
	if [ "$pinned" = "$current" ]; then
		printf '%-28s %-10s %-10s %s\n' "$name" "$pinned" "$current" 'fresh'
	else
		printf '%-28s %-10s %-10s %s\n' "$name" "$pinned" "$current" 'STALE — re-verify audit against current version'
		stale=$((stale + 1))
	fi
done

echo
if [ "$stale" -gt 0 ]; then
	echo "audit-freshness: $stale of $checked pinned audit(s) stale (go.mod moved past the audited pin)"
	exit 1
fi
echo "audit-freshness: all $checked pinned audit(s) fresh"
