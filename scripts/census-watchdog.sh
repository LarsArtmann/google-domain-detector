#!/usr/bin/env bash
# Census wave watchdog: detect a wedged wave runner from OUTSIDE the runner.
#
# The 2026-11-13 wave-A wedge (scan done 03:25, runner never logged
# "pass 1 finished", gate/snapshot never ran, found by luck hours later)
# is the failure this guards: run it detached next to every wave,
#
#   setsid nohup scripts/census-watchdog.sh reports/census/waves/<wave-id> \
#     >>reports/census/waves/<wave-id>/watchdog.log 2>&1 &
#
# It polls run1.jsonl growth; a stalled file with no completion line means
# either a wedged runner (process still alive — the wave-A signature) or a
# dead runner. Either way it logs a WATCHDOG ALARM into the wave's
# monitor.log and exits non-zero.
#
# verdicts (also the exit codes):
#   0  runner completed ("pass 1 finished" appeared in monitor.log)
#   1  WEDGED: jsonl stalled while the runner process is still alive
#   2  RUNNER DIED: runner process gone without a completion line
#
# usage: census-watchdog.sh <out-dir> [interval-sec] [stall-polls]
#   interval defaults 300, stall-polls defaults 3 (= 15 min of silence;
#   w4/10s throughput is never zero for that long on a healthy pass)
set -uo pipefail

if [ $# -lt 1 ]; then
	echo "usage: $0 <out-dir> [interval-sec] [stall-polls]" >&2
	exit 2
fi

out_dir=$1
interval=${2:-300}
stall_polls=${3:-3}
wave=$(basename "$out_dir")
monitor="$out_dir/monitor.log"
jsonl="$out_dir/run1.jsonl"

if [ ! -d "$out_dir" ]; then
	echo "census-watchdog: no such dir: $out_dir" >&2
	exit 2
fi

log() { printf '%s %s\n' "$(date '+%F %T')" "$*" >>"$monitor"; }

finished() { grep -q 'pass 1 finished' "$monitor" 2>/dev/null; }

runner_alive() {
	pgrep -f "census-wave.sh.*${wave}" >/dev/null 2>&1
}

log "watchdog: started (interval=${interval}s stall-polls=$stall_polls)"

size=0
stalled=0
while :; do
	if finished; then
		log "watchdog: runner completed normally — watchdog exiting"
		exit 0
	fi

	cur=0
	if [ -f "$jsonl" ]; then
		cur=$(stat -c %s "$jsonl" 2>/dev/null || echo 0)
	fi

	if [ "$cur" -eq "$size" ]; then
		stalled=$((stalled + 1))
	else
		stalled=0
	fi
	size=$cur

	if [ "$stalled" -ge "$stall_polls" ]; then
		quiet_min=$((stalled * interval / 60))
		if runner_alive; then
			log "WATCHDOG ALARM: run1.jsonl stalled ${quiet_min} min with no 'pass 1 finished' line and the wave runner STILL ALIVE — wedged post-scan (2026-11-13 wave-A signature). Inspect the runner pid, run scripts/census-gate.sh manually on the artifacts, then unwedge or kill the runner."
			exit 1
		fi
		log "WATCHDOG ALARM: run1.jsonl stalled ${quiet_min} min and the wave runner is GONE without a completion line — runner died mid-wave. Check monitor.log tail + run1.stderr, then relaunch per docs/census.md."
		exit 2
	fi

	sleep "$interval"
done
