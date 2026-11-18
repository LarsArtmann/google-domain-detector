#!/usr/bin/env bash
# Record the README demo: a 60-domain scan with an event store, then the
# event-sourcing quickstart (runs / show / replay). Produces:
#   - an asciinema cast (raw recording) at /tmp/gdd-demo.cast
#   - a rendered GIF at assets/demo.gif (via vhs; embed in README)
#
# Requires single-flight-free (it runs a real scan over the network):
#   pgrep -f 'gdd[^ ]* scan' must be empty before recording.
# Toolchain: nix shell nixpkgs#asciinema nixpkgs#vhs nixpkgs#ttyd nixpkgs#ffmpeg
#
# Usage: scripts/demo-record.sh [input-file]
#   input defaults to the first 60 lines of the wave-B manifest.

set -euo pipefail
here=$(cd "$(dirname "$0")" && pwd)
repo=$(cd "$here/.." && pwd)
input=${1:-$repo/reports/census/waves/tranco1m-b.txt}
out_cast=${2:-/tmp/gdd-demo.cast}
work=/tmp/gdd-demo-$(date '+%H%M%S')

if pgrep -f 'gdd[^ ]* scan' >/dev/null 2>&1; then
	echo "demo: REFUSED - a census scan is running (single-flight law, docs/census.md)" >&2
	exit 3
fi
if [ ! -e "$input" ]; then
	echo "demo: no such input: $input" >&2
	exit 2
fi

mkdir -p "$work"
head -60 "$input" >"$work/demo60.txt"

export GOCACHE=${GOCACHE:-$HOME/.cache/go-build-demo} GOMODCACHE=${GOMODCACHE:-$HOME/go/pkg/mod}
(cd "$repo" && go build -o "$work/gdd-demo" ./cmd/google-domain-detector)

GDD="$work/gdd-demo"
DB="$work/demo.db"
first_domain=$(head -1 "$work/demo60.txt")

nix shell nixpkgs#asciinema -c asciinema rec --quiet --overwrite -c "bash -s" "$out_cast" <<DEMO
set -u
prompt() { printf '\e[1;32m\$\e[0m %s\n' "\$*"; }
run() { prompt "\$*"; eval "\$*"; echo; }

clear
echo "google-domain-detector - detect Google Workspace / GCP usage from public DNS alone"
echo

prompt 'cut a 60-domain slice and scan it with an event store'
echo '\$ gdd scan --input demo60.txt --no-ranges --workers 4 --timeout 10s --event-store demo.db'
$GDD scan --input "$work/demo60.txt" --no-ranges --workers 4 --timeout 10s \\
	--event-store "$DB" --format table 2>&1 | head -40
echo "  ... (60 domains total)"
echo

run '\$ gdd runs --event-store demo.db'
$GDD runs --event-store "$DB" 2>&1 | head -8
echo

run "\$ gdd show $first_domain --event-store demo.db"
$GDD show "$first_domain" --event-store "$DB" 2>&1 | head -12
echo

run '\$ gdd replay --event-store demo.db   # journal alone, no DNS'
$GDD replay --event-store "$DB" 2>&1 | head -25
echo
echo "Every observation is an immutable event - the report is replayable from the journal alone."
DEMO

cat >"$work/demo.tape" <<TAPE
Output assets/demo.gif
Set FontSize 20
Set Width 1200
Set Height 800
Set Shell bash
Set TypingSpeed 3ms
Type "echo google-domain-detector: 60-domain scan, replayable journal"
Enter
Sleep 800ms
Type "$GDD scan --input $work/demo60.txt --no-ranges --workers 4 --timeout 10s --event-store $DB --format table"
Enter
Sleep 25s
Type "$GDD runs --event-store $DB"
Enter
Sleep 2s
Type "$GDD show $first_domain --event-store $DB"
Enter
Sleep 2s
Type "$GDD replay --event-store $DB"
Enter
Sleep 3s
TAPE

(cd "$repo" && nix shell nixpkgs#vhs nixpkgs#ttyd nixpkgs#ffmpeg -c vhs "$work/demo.tape")

echo "cast written: $out_cast"
echo "gif written:  $repo/assets/demo.gif"
echo "README embed: ![](assets/demo.gif) after the quickstart block"
