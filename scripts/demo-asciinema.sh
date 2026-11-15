#!/usr/bin/env bash
# Record the README demo: a 60-domain scan with an event store, then the
# event-sourcing quickstart (runs / show / replay) as an asciinema cast.
#
# The cast lands in /tmp/gdd-demo.cast; convert to GIF with agg:
#   nix run nixpkgs#agg -- /tmp/gdd-demo.cast assets/demo.gif
#
# Requires single-flight-free (it runs a real scan over the network):
#   pgrep -f 'gdd[^ ]* scan' must be empty before recording.
#
# Usage: scripts/demo-asciinema.sh [input-file]
#   input defaults to the first 60 lines of the wave-B manifest.

set -euo pipefail
here=$(cd "$(dirname "$0")" && pwd)
repo=$(cd "$here/.." && pwd)
input=${1:-$repo/reports/census/waves/tranco1m-b.txt}
out_cast=${2:-/tmp/gdd-demo.cast}
work=/tmp/gdd-demo-$(date '+%H%M%S')

if pgrep -f 'gdd[^ ]* scan' >/dev/null 2>&1; then
	echo "demo: REFUSED — a census scan is running (single-flight law, docs/census.md)" >&2
	exit 3
fi
if [ ! -e "$input" ]; then
	echo "demo: no such input: $input" >&2
	exit 2
fi

mkdir -p "$work"
head -60 "$input" > "$work/demo60.txt"

export GOCACHE=${GOCACHE:-$HOME/.cache/go-build-demo} GOMODCACHE=${GOMODCACHE:-$HOME/go/pkg/mod}
( cd "$repo" && go build -o "$work/gdd-demo" ./cmd/google-domain-detector )

GDD="$work/gdd-demo"
DB="$work/demo.db"

record() {
	asciinema rec --quiet --overwrite -c "$1" "$out_cast"
}

record "GDD=$GDD DB=$DB WORK=$work bash -s" <<'DEMO'
set -u
prompt() { printf '\e[1;32m$\e[0m %s\n' "$*"; }
run() { prompt "$*"; eval "$*"; echo; }

clear
echo "google-domain-detector — detect Google Workspace / GCP usage from public DNS alone"
echo

prompt 'cut a 60-domain slice and scan it with an event store'
echo '$ gdd scan --input demo60.txt --no-ranges --workers 4 --timeout 10s \'
echo '       --event-store demo.db --format table'
"$GDD" scan --input "$WORK/demo60.txt" --no-ranges --workers 4 --timeout 10s \
	--event-store "$DB" --format table 2>&1 | head -40
echo "  ... (60 domains total)"
echo

run '$ gdd runs --event-store demo.db'
"$GDD" runs --event-store "$DB" 2>&1 | head -8
echo

first_domain=$(head -1 "$WORK/demo60.txt")

run "\$ gdd show $first_domain --event-store demo.db"
"$GDD" show "$first_domain" --event-store "$DB" 2>&1 | head -12
echo

run '$ gdd replay --event-store demo.db   # journal alone, no DNS'
"$GDD" replay --event-store "$DB" 2>&1 | head -25
echo
echo "Every observation is an immutable event — the report is replayable from the journal alone."
DEMO

echo "cast written: $out_cast"
echo "GIF: nix run nixpkgs#agg -- $out_cast assets/demo.gif"
