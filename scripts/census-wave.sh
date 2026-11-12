#!/usr/bin/env bash
# Census wave runner: one wave pass under the single-flight law, then
# the E16 cleanup loop, then the D3 gate — everything logged so the run
# survives the launching session (run detached: setsid nohup).
#
# usage: census-wave.sh <gdd-binary> <store> <wave-id> <manifest> <out-dir> [workers] [timeout]
#   workers defaults to 4, timeout to 10s (the 2026-11-11 sustained-load
#   decision; see docs/census.md). Progress lands in <out-dir>/monitor.log,
#   the pass in <out-dir>/run1.{jsonl,stderr}, cleanup rounds in
#   <out-dir>/<wave-id>-c<N>.{jsonl,stderr}. On completion (either path)
#   it snapshots the journal via census-snapshot.sh into
#   <store-dir>/snapshots/ (§g3 default: local tarball per wave).
# env: GDD_RESOLVER (127.0.0.1:5353), GDD_CLEANUP_BAR_PCT (0.5),
#   GDD_WARN_PCT (2) — mid-pass unknown-rate WARNING threshold (D3 bar;
#   2 = percent). Set GDD_WARN_PCT=0 to disable the warning.
set -uo pipefail

if [ $# -lt 5 ]; then
	echo "usage: $0 <gdd-binary> <store> <wave-id> <manifest> <out-dir> [workers] [timeout]" >&2
	exit 2
fi

gdd_bin=$1
store=$2
wave=$3
manifest=$4
out_dir=$5
workers=${6:-4}
timeout_budget=${7:-10s}
resolver=${GDD_RESOLVER:-127.0.0.1:5353}
bar=${GDD_CLEANUP_BAR_PCT:-0.5}
warn_pct=${GDD_WARN_PCT:-2}
here=$(cd "$(dirname "$0")" && pwd)

for f in "$gdd_bin" "$store" "$manifest"; do
	if [ ! -e "$f" ]; then
		echo "census-wave: no such path: $f" >&2
		exit 2
	fi
done

log() { printf '%s %s\n' "$(date '+%F %T')" "$*" >>"$out_dir/monitor.log"; }

snapshot() {
	local snap_dir snap_log
	snap_dir=$(dirname "$store")/snapshots
	if snap_log=$("$here/census-snapshot.sh" "$store" "$snap_dir" "$wave-complete-$(date '+%Y%m%d-%H%M')" 2>&1); then
		log "journal snapshot: $snap_log"
	else
		log "journal snapshot FAILED (journal remains the source of truth; re-run scripts/census-snapshot.sh manually): $snap_log"
	fi
}

mkdir -p "$out_dir"
: >"$out_dir/monitor.log"

if pgrep -f 'gdd scan' >/dev/null 2>&1; then
	log "REFUSED: another census scan is running (single-flight law, docs/census.md)"
	exit 3
fi

log "wave $wave pass 1 starting: workers=$workers timeout=$timeout_budget resolver=$resolver input=$manifest"

progress() {
	local total mixed unknown pct
	total=$(wc -l <"$out_dir/run1.jsonl" 2>/dev/null || echo 0)
	mixed=$(jq -r '.error_kind // "none"' "$out_dir/run1.jsonl" 2>/dev/null | sort | uniq -c | tr '\n' ';')
	log "progress: $total scanned — $mixed"
	if [ "$total" -gt 0 ] 2>/dev/null && awk -v w="$warn_pct" 'BEGIN { exit !(w > 0) }'; then
		unknown=$(jq -r '.error_kind // "none"' "$out_dir/run1.jsonl" 2>/dev/null | grep -c -v -E '^(none|nxdomain)$')
		pct=$(awk -v u="$unknown" -v t="$total" 'BEGIN { printf "%.2f", 100 * u / t }')
		if awk -v p="$pct" -v w="$warn_pct" 'BEGIN { exit !(p > w) }'; then
			log "WARNING: unknown rate ${pct}% above bar ${warn_pct}% at $total scanned — check unbound health (scripts/unbound.sh status) and error mix above"
		fi
	fi
}
(while :; do
	sleep 1800
	progress
done) &
poller=$!

"$gdd_bin" scan --no-ranges --workers "$workers" --timeout "$timeout_budget" \
	--resolver "$resolver" --event-store "$store" --wave "$wave" \
	--input "$manifest" --resume latest --format jsonl \
	>"$out_dir/run1.jsonl" 2>"$out_dir/run1.stderr"
scan_rc=$?
kill "$poller" 2>/dev/null
log "wave $wave pass 1 finished: exit=$scan_rc"
grep -E '^[0-9]+ domains:' "$out_dir/run1.stderr" | tail -1 >>"$out_dir/monitor.log"

"$here/census-gate.sh" "$out_dir/run1.stderr" >>"$out_dir/monitor.log" 2>&1
gate_rc=$?

if [ "$gate_rc" -ne 0 ]; then
	log "pass-1 gate FAIL — running cleanup loop (E16)"
	"$here/census-cleanup.sh" "$gdd_bin" "$store" "$wave" "$out_dir/run1.jsonl" >>"$out_dir/monitor.log" 2>&1
	cleanup_rc=$?
	log "cleanup loop exit=$cleanup_rc — residual gate verdict above; stable-SERVFAIL assessment (census-gate.sh --servfail-stable) is the operator's explicit call"
	snapshot
	exit "$cleanup_rc"
fi

log "pass-1 gate PASS — wave $wave done (no cleanup needed)"
snapshot
exit 0
