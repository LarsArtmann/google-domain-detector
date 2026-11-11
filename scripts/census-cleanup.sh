#!/usr/bin/env bash
# Census cleanup loop (E16): drive a wave's unknown-class errors to under
# the D3 cleanup bar. Unknown-class = timeout/servfail/canceled/other
# (everything that is NOT an authoritative NXDOMAIN — see census-gate.sh).
#
# Each round re-scans only the still-unexplained domains at LOWER workers
# (round 1: 4, later rounds: 2) with a longer timeout, as a new journal
# run under wave id "<wave>-c<N>", until the residual is under
# unknown-bar% of the ORIGINAL input size or max-rounds is hit.
#
# The percentage base is deliberately the original input size, not the
# shrinking round size: a cleanup round's own unknown-rate is meaningless
# (its input is 100% prior errorers); what the D3 bar bounds is the wave's
# total unexplained fraction (E10 pattern: 13/7204 = 0.18%).
#
# usage: census-cleanup.sh <gdd-binary> <event-store> <wave-id> <wave-jsonl> [max-rounds]
#   max-rounds defaults to 3
# env: GDD_RESOLVER (127.0.0.1:5353), GDD_CLEANUP_TIMEOUT (12s),
#      GDD_CLEANUP_BAR_PCT (0.5)
set -euo pipefail

if [ $# -lt 4 ]; then
	echo "usage: $0 <gdd-binary> <event-store> <wave-id> <wave-jsonl> [max-rounds]" >&2
	exit 2
fi

gdd_bin=$1
store=$2
wave=$3
in_jsonl=$4
max_rounds=${5:-3}

resolver=${GDD_RESOLVER:-127.0.0.1:5353}
timeout_budget=${GDD_CLEANUP_TIMEOUT:-12s}
bar=${GDD_CLEANUP_BAR_PCT:-0.5}

for f in "$gdd_bin" "$store" "$in_jsonl"; do
	if [ ! -e "$f" ]; then
		echo "census-cleanup: no such path: $f" >&2
		exit 2
	fi
done

out_dir=$(dirname "$in_jsonl")
work_list=$(mktemp /tmp/census-cleanup.XXXXXX.txt)
trap 'rm -f "$work_list"' EXIT

base_total=$(wc -l <"$in_jsonl")
if [ "$base_total" -eq 0 ]; then
	echo "census-cleanup: empty input: $in_jsonl" >&2
	exit 2
fi

current=$in_jsonl
residual=$(jq -r 'select(.error_kind == "timeout" or .error_kind == "servfail" or .error_kind == "canceled" or .error_kind == "other") | .domain' "$current" | wc -l)

round=0
while [ "$residual" -gt 0 ] && [ "$round" -lt "$max_rounds" ]; do
	round=$((round + 1))
	if [ "$round" -eq 1 ]; then workers=4; else workers=2; fi

	jq -r 'select(.error_kind == "timeout" or .error_kind == "servfail" or .error_kind == "canceled" or .error_kind == "other") | .domain' "$current" >"$work_list"
	round_jsonl=$out_dir/${wave}-c${round}.jsonl
	round_stderr=$out_dir/${wave}-c${round}.stderr

	echo "census-cleanup: round $round — $residual unknown(s), workers $workers, timeout $timeout_budget"
	"$gdd_bin" scan --no-ranges --workers "$workers" --timeout "$timeout_budget" \
		--resolver "$resolver" --event-store "$store" --wave "${wave}-c${round}" \
		--input "$work_list" --format jsonl >"$round_jsonl" 2>"$round_stderr"

	current=$round_jsonl
	residual=$(jq -r 'select(.error_kind == "timeout" or .error_kind == "servfail" or .error_kind == "canceled" or .error_kind == "other") | .domain' "$current" | wc -l)
	pct=$(awk -v r="$residual" -v t="$base_total" 'BEGIN { printf "%.2f", r * 100.0 / t }')
	echo "census-cleanup: after round $round — residual $residual/$base_total (${pct}%)"
done

pct=$(awk -v r="$residual" -v t="$base_total" 'BEGIN { printf "%.2f", r * 100.0 / t }')
under=$(awk -v p="$pct" -v b="$bar" 'BEGIN { print (p < b) ? 1 : 0 }')
if [ "$residual" -eq 0 ]; then
	echo "census-cleanup: CLEAN — zero unknowns after $round round(s)"
	exit 0
elif [ "$under" -eq 1 ]; then
	echo "census-cleanup: PASS — residual ${residual}/${base_total} (${pct}%) under bar ${bar}%"
	exit 0
fi
echo "census-cleanup: STILL OVER BAR — residual ${residual}/${base_total} (${pct}%) vs bar ${bar}% after $round round(s)"
echo "census-cleanup: next levers: another round, then assess stable-SERVFAIL (census-gate.sh --servfail-stable; D3 refinement in docs/census.md)"
exit 1
