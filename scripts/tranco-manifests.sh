#!/usr/bin/env bash
# Freeze Tranco top-1M input into census wave manifests (census plan T10).
#
# Input:  the raw Tranco CSV ("rank,domain" per line, permanent URL
#         https://tranco-list.eu/top-1m.csv.zip; the list ID for this
#         freeze is recorded in docs/census.md).
# Output: reports/census/waves/tranco1m-{a,b,c}.txt — frozen manifest
#         files keyed by rank band (A: 1-100k, B: 100k-500k, C: 500k-1M)
#         plus a checksums file. Waves resume from the journal, so these
#         files are the ONLY definition of what a wave covers — never
#         re-cut them from a fresh Tranco download mid-census.
#
# usage: tranco-manifests.sh <tranco.csv>
set -euo pipefail

if [ $# -ne 1 ]; then
    echo "usage: $0 <tranco.csv>" >&2
    exit 2
fi

csv=$1
waves_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)/reports/census/waves"
mkdir -p "$waves_dir"

if [ ! -f "$csv" ]; then
    echo "tranco-manifests: no such csv: $csv" >&2
    exit 2
fi

# CRLF-safe; strip the rank column; drop blanks.
domains=$(mktemp)
trap 'rm -f "$domains"' EXIT
tr -d '\r' <"$csv" | sed -E 's/^[0-9]+,//' | sed '/^$/d' >"$domains"

total=$(wc -l <"$domains")
if [ "$total" -ne 1000000 ]; then
    echo "tranco-manifests: expected 1000000 domains, got $total — refusing to freeze a malformed list" >&2
    exit 1
fi

sed -n '1,100000p'     "$domains" >"$waves_dir/tranco1m-a.txt"
sed -n '100001,500000p' "$domains" >"$waves_dir/tranco1m-b.txt"
sed -n '500001,1000000p' "$domains" >"$waves_dir/tranco1m-c.txt"

# Sanity: disjoint, complete, correctly sized (census plan 12.3).
a=$(wc -l <"$waves_dir/tranco1m-a.txt")
b=$(wc -l <"$waves_dir/tranco1m-b.txt")
c=$(wc -l <"$waves_dir/tranco1m-c.txt")
if [ $((a + b + c)) -ne "$total" ] || [ "$a" -ne 100000 ] || [ "$b" -ne 400000 ] || [ "$c" -ne 500000 ]; then
    echo "tranco-manifests: band sizes wrong (a=$a b=$b c=$c, total $total)" >&2
    exit 1
fi
if [ "$(cat "$waves_dir"/tranco1m-{a,b,c}.txt | sort -u | wc -l)" -ne "$total" ]; then
    echo "tranco-manifests: bands overlap or duplicate domains — refusing" >&2
    exit 1
fi

( cd "$waves_dir" && sha256sum tranco1m-{a,b,c}.txt > MANIFEST.sha256 )

echo "frozen $(basename "$csv") → $waves_dir (list id must be recorded in docs/census.md)"
( cd "$waves_dir" && cat MANIFEST.sha256 )
