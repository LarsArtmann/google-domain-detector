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
#   GDD_NO_PREFLIGHT=1 skips the upstream pre-flight (test harnesses
#   only; never for production waves). GDD_PREFLIGHT_TRANSPORT=tcp probes
#   upstreams over TCP — set it for waves on the TCP-upstream forwarder
#   (unbound-tcp.conf) so a UDP-only outage cannot block a launch that
#   would be immune to it.
# exit: 0 wave done/gate pass · 3 single-flight refusal · 4 coverage gap
#   · 5 pre-flight failure (upstream or forwarder dead — see docs/census.md
#   "outage fallback" before relaunching; never reuse a cache that ran
#   while upstream was dead)
#
# Resume policy (2026-11-12, wave-A post-mortem): a FRESH out-dir never
# passes --resume — `--resume latest` silently skips whatever the latest
# STARTED run (any aborted soak/pilot!) had journaled, and wave A lost
# 2,731 of 100,000 domains to exactly that (docs/census.md "resume
# hazard"). A RETRY of the same out-dir (run-id file present from a
# previous attempt) resumes from THAT run explicitly, with the gate's
# --expect-skip set to its observed-domain count so coverage still
# reconciles. The run id is captured from stderr after every attempt
# (the "run <id> recorded" line prints even on abort).
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

if pgrep -f 'gdd[^ ]* scan' >/dev/null 2>&1; then
	log "REFUSED: another census scan is running (single-flight law, docs/census.md)"
	exit 3
fi

# Real-upstream pre-flight (2026-11-12 UDP:53 outage lesson): a forwarder
# answering from cache proves NOTHING about upstream health — during the
# outage `unbound.sh status` was green while every fresh recursion died and
# the wave burned hours on fabricated SERVFAILs. Two direct upstream probes
# and one forwarder probe must all return a DNS status before any pass may
# start. SERVFAIL counts as failure: an upstream that answers but refuses
# is as useless for a census as a dead one.
preflight() {
	local upstream status resolver_host resolver_port transport_args=()
	if [ "${GDD_NO_PREFLIGHT:-0}" = "1" ]; then
		log "pre-flight SKIPPED via GDD_NO_PREFLIGHT=1 (operator override — never for production waves)"
		return 0
	fi
	# Waves on the TCP-upstream forwarder (unbound-tcp.conf) must probe
	# upstreams over TCP too — immune to a UDP-only outage (2026-11-12:
	# UDP:53 died while TCP:53 worked).
	if [ "${GDD_PREFLIGHT_TRANSPORT:-udp}" = "tcp" ]; then
		transport_args=(+tcp)
		log "pre-flight: probing upstreams over TCP (GDD_PREFLIGHT_TRANSPORT=tcp)"
	fi
	for upstream in 9.9.9.9 1.1.1.1; do
		status=$(dig +time=3 +tries=1 "${transport_args[@]+"${transport_args[@]}"}" "@$upstream" google.com A 2>/dev/null | grep -oE 'status: (NOERROR|NXDOMAIN)' | head -1)
		if [ -z "$status" ]; then
			log "pre-flight FAIL: upstream $upstream unresponsive on UDP:53 — outbound DNS outage suspected. Do NOT launch; if switching to the TCP forwarder (docs/census.md 'outage fallback'), restart it fresh and never reuse a cache that ran while upstream was dead."
			return 1
		fi
		log "pre-flight: upstream $upstream OK ($status)"
	done
	# dig wants "@host -p port", not "@host:port" — split GDD_RESOLVER
	# (bare host, host:port, or [v6]:port) before probing.
	resolver_host=$resolver
	resolver_port=53
	if [[ $resolver =~ ^\[(.+)\]:([0-9]+)$ ]]; then
		resolver_host=${BASH_REMATCH[1]}
		resolver_port=${BASH_REMATCH[2]}
	elif [[ $resolver =~ ^([^:]+):([0-9]+)$ ]]; then
		resolver_host=${BASH_REMATCH[1]}
		resolver_port=${BASH_REMATCH[2]}
	fi
	status=$(dig +time=3 +tries=1 "@$resolver_host" -p "$resolver_port" google.com A 2>/dev/null | grep -oE 'status: NOERROR' | head -1)
	if [ -z "$status" ]; then
		log "pre-flight FAIL: forwarder $resolver unresponsive — check scripts/unbound.sh status and restart the forwarder before any wave."
		return 1
	fi
	log "pre-flight: forwarder $resolver OK ($status)"
	return 0
}

if ! preflight; then
	exit 5
fi

log "wave $wave pass 1 starting: workers=$workers timeout=$timeout_budget resolver=$resolver input=$manifest"

resume_args=()
expect_skip=0
if [ -s "$out_dir/run-id" ]; then
	prior_run=$(cat "$out_dir/run-id")
	resume_args=(--resume "$prior_run")
	log "retry: resuming run $prior_run (domains it observed are skipped by design)"
	if command -v sqlite3 >/dev/null 2>&1; then
		observed=$(sqlite3 "$store" "SELECT COUNT(DISTINCT stream_id) FROM meta_stream_log WHERE value LIKE '%scan.domain_observed%' AND CAST(payload AS TEXT) LIKE '%$prior_run%'" 2>/dev/null || true)
		if [ -n "$observed" ] && [ "$observed" -gt 0 ] 2>/dev/null; then
			expect_skip=$observed
			log "retry: gate expect-skip = $expect_skip (journal count for run $prior_run)"
		fi
	fi
else
	log "fresh pass: no --resume (immune to soak/pilot journal poisoning; docs/census.md 'resume hazard')"
fi

last_size=0
stalled_polls=0
progress() {
	local total mixed unknown pct cur
	cur=$(stat -c %s "$out_dir/run1.jsonl" 2>/dev/null || echo 0)
	if [ "$cur" -eq "$last_size" ]; then
		stalled_polls=$((stalled_polls + 1))
		if [ "$stalled_polls" -ge 2 ]; then
			log "HEARTBEAT STALL: run1.jsonl unchanged across $stalled_polls polls ($((stalled_polls * 30)) min) — the 2026-11-13 wave-A wedge signature; check the runner pid and scripts/census-watchdog.sh"
		fi
	else
		stalled_polls=0
	fi
	last_size=$cur
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
	--input "$manifest" --format jsonl ${resume_args[@]+"${resume_args[@]}"} \
	>"$out_dir/run1.jsonl" 2>"$out_dir/run1.stderr"
scan_rc=$?
kill "$poller" 2>/dev/null
log "wave $wave pass 1 finished: exit=$scan_rc"
grep -E '^[0-9]+ domains:' "$out_dir/run1.stderr" | tail -1 >>"$out_dir/monitor.log"

run_id=$(grep -oE 'run [0-9A-Z]{10,32} recorded' "$out_dir/run1.stderr" | tail -1 | awk '{print $2}')
if [ -n "$run_id" ]; then
	printf '%s\n' "$run_id" >"$out_dir/run-id"
	log "run id: $run_id (recorded in $out_dir/run-id for explicit retries)"
fi

"$here/census-gate.sh" --manifest "$manifest" --expect-skip "$expect_skip" "$out_dir/run1.stderr" >>"$out_dir/monitor.log" 2>&1
gate_rc=$?

if [ "$gate_rc" -eq 4 ]; then
	log "pass-1 COVERAGE GAP — do NOT scale and do NOT clean up: reconcile the missing domains against the journal (census.md 'resume hazard'), then retry this out-dir (explicit --resume via run-id) or patch-scan the gap"
	snapshot
	exit 4
fi

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
