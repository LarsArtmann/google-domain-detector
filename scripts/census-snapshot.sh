#!/usr/bin/env bash
# Census journal snapshot (§g3 default: local tarball per completed wave).
# Uses sqlite3's online backup API, so the snapshot is WAL-consistent even
# while a wave is mid-scan (single-flight law still holds for SCANS; this
# only reads). Output: <snapshots-dir>/<label>.tar.gz + .sha256 sidecar.
#
# usage: census-snapshot.sh <store> <snapshots-dir> [label]
#   label defaults to a timestamp; pass e.g. tranco1m-a-complete at wave
#   boundaries so the snapshot name says which census state it holds.
# exit 0 = snapshot written and verified; 1 = backup/verify failed; 2 = usage.
set -euo pipefail

if [ $# -lt 2 ] || [ $# -gt 3 ]; then
    echo "usage: $0 <store> <snapshots-dir> [label]" >&2
    exit 2
fi

store=$1
snap_dir=$2
label=${3:-$(date '+%Y%m%d-%H%M%S')}

if [ ! -f "$store" ]; then
    echo "census-snapshot: no such store: $store" >&2
    exit 2
fi
command -v sqlite3 >/dev/null 2>&1 || {
    echo "census-snapshot: sqlite3 not found (needed for the online backup API)" >&2
    exit 1
}

mkdir -p "$snap_dir"
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

# Online backup: consistent snapshot including WAL content, safe under load.
sqlite3 "$store" ".backup '$tmp/census.db'"

# Verify the snapshot is a sound database before shipping it.
integrity=$(sqlite3 "$tmp/census.db" 'PRAGMA integrity_check;' 2>&1)
if [ "$integrity" != "ok" ]; then
    echo "census-snapshot: integrity_check failed on snapshot: $integrity" >&2
    exit 1
fi

out="$snap_dir/$label.tar.gz"
tar -czf "$out" -C "$tmp" census.db
sha256sum "$out" >"$out.sha256"

src_bytes=$(wc -c <"$tmp/census.db" | tr -d ' ')
snap_bytes=$(wc -c <"$out" | tr -d ' ')
printf 'census-snapshot: %s — db %s bytes, tar %s bytes, sha256 sidecar next to it\n' \
    "$out" "$src_bytes" "$snap_bytes"
