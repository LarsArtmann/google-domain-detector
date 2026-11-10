#!/usr/bin/env bash
# Census pilot gate (G6): decide from a finished scan whether it is safe
# to scale up 10x. Fails when unknown-class errors (timeout + servfail +
# canceled + other — everything that is NOT an authoritative NXDOMAIN)
# exceed the budget. NXDOMAIN is domain truth (an authoritative "this
# name does not exist"), the rest are tool-environment states: they say
# nothing about the domain and must be retried or re-tuned, never scaled
# past (census plan guard 4, D3 bar).
#
# Input: either a scan's stderr file (its one-line run summary) or a
# scan's jsonl output (error_kind counted per line).
# usage: census-gate.sh <stderr-or-jsonl-file> [max-unknown-pct]
#   max-unknown-pct defaults to 2 (the D3 bar); 0.5 works for cleanup loops
set -euo pipefail

if [ $# -lt 1 ]; then
    echo "usage: $0 <stderr-or-jsonl-file> [max-unknown-pct]" >&2
    exit 2
fi

file=$1
budget=${2:-2}

if [ ! -f "$file" ]; then
    echo "census-gate: no such file: $file" >&2
    exit 2
fi

pass_fail() {
    local total=$1 nxdomain=$2 unknown=$3
    awk -v total="$total" -v nx="$nxdomain" -v unk="$unknown" -v budget="$budget" '
        BEGIN {
            if (total <= 0) { print "census-gate: FAIL (no domains counted)"; exit 1 }
            rate = unk * 100.0 / total
            printf "domains %d | nxdomain %d (%.2f%%, domain truth) | unknown %d (%.2f%%, budget %.2f%%)\n", total, nx, nx*100.0/total, unk, rate, budget
            if (rate >= budget) { print "census-gate: FAIL — resolver/tool health below bar, do NOT scale"; exit 1 }
            print "census-gate: PASS"
        }'
}

summary_pattern='^[0-9]+ domains: workspace certain [0-9]+ / likely [0-9]+, gcp [0-9]+, errors [0-9]+'

if grep -Eq "$summary_pattern" "$file"; then
    # stderr run-summary form:
    # "N domains: workspace certain X / likely Y, gcp Z, errors E (nxdomain a, timeout b, servfail c, canceled d, other e)"
    line="$(grep -E "$summary_pattern" "$file" | tail -1)"
    read -r total nx timeouts servfail canceled other <<<"$(printf '%s\n' "$line" | sed -E '
        s/^([0-9]+) domains:.*errors [0-9]+ \(nxdomain ([0-9]+), timeout ([0-9]+), servfail ([0-9]+), canceled ([0-9]+), other ([0-9]+)\).*/\1 \2 \3 \4 \5 \6/')"
    if [ -z "${total:-}" ] || [ "$total" = "$line" ]; then
        echo "census-gate: found summary line but could not parse it: $line" >&2
        exit 2
    fi
    unknown=$((timeouts + servfail + canceled + other))
    pass_fail "$total" "$nx" "$unknown"
elif grep -q '"error_kind"' "$file"; then
    # jsonl form: count per-line error_kind values
    read -r total errors nx timeouts servfail canceled other <<<"$(jq -r '[.error_kind // "none"] | @tsv' "$file" | awk -F'\t' '
        { total++; k=$1 }
        k=="none"    { ok++ }
        k=="nxdomain" { nx++ }
        k=="timeout"  { t++ }
        k=="servfail" { s++ }
        k=="canceled" { c++ }
        k=="other"    { o++ }
        END { printf "%d %d %d %d %d %d %d\n", total, total-(nx+t+s+c+o), nx+0, t+0, s+0, c+0, o+0 }')"
    unknown=$((timeouts + servfail + canceled + other))
    pass_fail "$total" "$nx" "$unknown"
else
    echo "census-gate: $file is neither a run summary (stderr) nor jsonl output" >&2
    exit 2
fi
