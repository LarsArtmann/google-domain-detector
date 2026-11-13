#!/usr/bin/env bash
# Version-freshness guard (TODO_LIST "Version-freshness guard"): the
# library audits under docs/review/libs/ pin the version they reviewed
# ("Status ... pinned vX.Y.Z"). scripts/audit-freshness.sh compares those
# pins against go.mod (drift inside THIS repo); this guard compares them
# against the upstream module proxy: a module whose pinned audit is older
# than the latest published tag is flagged so the audit gets refreshed
# before its verdicts mislead. Runs in CI; needs proxy network access.
#
# Exit 0 = every pinned audit is current upstream; exit 1 = at least one
# stale (each named); exit 2 = usage trouble. Proxy-unreachable modules
# SKIP (with a note) rather than red the build — a network blip is not a
# staleness verdict.
#
# usage: lib-freshness.sh [repo-root]   (defaults to the script's repo)
set -euo pipefail

root=${1:-$(cd "$(dirname "$0")/.." && pwd)}
libs_dir=$root/docs/review/libs

if [ ! -d "$libs_dir" ]; then
	echo "lib-freshness: no docs/review/libs under: $root" >&2
	exit 2
fi

stale=0
checked=0
printf '%-28s %-10s %-12s %s\n' 'AUDIT' 'PINNED' 'LATEST' 'VERDICT'
for audit in "$libs_dir"/*.md; do
	name=$(basename "$audit" .md)

	module=$(sed -n 's/^>.*Module: `\([^`]*\)`.*/\1/p' "$audit" | head -1)
	pinned=$(sed -n 's/.*pinned \(v[0-9][^ *]*\).*/\1/p' "$audit" | head -1)

	if [ -z "$module" ] || [ -z "$pinned" ]; then
		printf '%-28s %-10s %-12s %s\n' "$name" '-' '-' 'SKIP (no version pin)'
		continue
	fi

	# Latest tag straight from the proxy; run inside the repo so module
	# mode has a go.mod context (the queried module need not be a dep).
	latest=$(cd "$root" && go list -m -versions "$module" 2>/dev/null | awk '{print $NF}')
	if [ -z "$latest" ]; then
		printf '%-28s %-10s %-12s %s\n' "$name" "$pinned" '-' 'SKIP (proxy unreachable or module unresolvable)'
		continue
	fi

	checked=$((checked + 1))
	if [ "$(printf '%s\n%s\n' "$pinned" "$latest" | sort -V | tail -1)" = "$pinned" ]; then
		printf '%-28s %-10s %-12s %s\n' "$name" "$pinned" "$latest" 'fresh'
	else
		printf '%-28s %-10s %-12s %s\n' "$name" "$pinned" "$latest" 'STALE — audit predates the latest release'
		stale=$((stale + 1))
	fi
done

echo
if [ "$stale" -gt 0 ]; then
	echo "lib-freshness: $stale of $checked pinned audit(s) stale upstream"
	exit 1
fi
echo "lib-freshness: all $checked pinned audit(s) fresh upstream"
