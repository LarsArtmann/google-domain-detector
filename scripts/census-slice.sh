#!/usr/bin/env bash
# Census pilot helper: cut a slice of the first N domains off a list.
# CRLF-safe (Tranco lists ship with Windows line endings on some mirrors).
# usage: census-slice.sh <domain-list> <n> [outfile]
#   outfile defaults to stdout
set -euo pipefail

if [ $# -lt 2 ]; then
	echo "usage: $0 <domain-list> <n> [outfile]" >&2
	exit 2
fi

list=$1
count=$2
out=${3:-/dev/stdout}

if [ ! -f "$list" ]; then
	echo "census-slice: no such list: $list" >&2
	exit 2
fi
if ! [[ $count =~ ^[0-9]+$ ]] || [ "$count" -eq 0 ]; then
	echo "census-slice: slice size must be a positive integer, got: $count" >&2
	exit 2
fi

# head first, then tr: under pipefail the reverse order makes head's early
# exit SIGPIPE tr and fail the script (exit 141) even though the slice is
# complete.
head -n "$count" <"$list" | tr -d '\r' >"$out"
