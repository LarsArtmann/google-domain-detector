#!/usr/bin/env bash
# Census pilot gate (G6): decide from a finished scan whether it is safe
# to scale up 10x. Fails when unknown-class errors (timeout + servfail +
# canceled + other — everything that is NOT an authoritative NXDOMAIN)
# exceed the budget. NXDOMAIN is domain truth (an authoritative "this
# name does not exist"), the rest are tool-environment states: they say
# nothing about the domain and must be retried or re-tuned, never scaled
# past (census plan guard 4, D3 bar).
#
# --servfail-stable: D3 refinement — reclassify SERVFAIL as explained
# (domain-side broken delegation) instead of unknown. ONLY valid after
# the SERVFAILs proved stable across >= 2 cleanup rounds through a
# verified-healthy resolver (docs/census.md, E10 decision); the flag makes
# the reclassification explicit in the gate output instead of silent.
#
# --manifest FILE / --expect-skip N: coverage assertion — unique manifest
# lines must reconcile with scanned + unparseable-skipped (+ expected)
# domains. Guards against SILENT coverage holes: `--resume latest` skips
# every domain the (possibly aborted soak/pilot) source run journaled and
# the scan prints nothing about it — wave A lost 2,731 of 100,000 domains
# exactly this way (2026-11-12; docs/census.md "resume hazard"). A
# coverage gap exits 4 (health FAIL stays exit 1, the cleanup remedy).
#
# Input: either a scan's stderr file (its one-line run summary) or a
# scan's jsonl output (error_kind counted per line).
# usage: census-gate.sh [--servfail-stable] [--manifest FILE] [--expect-skip N] <stderr-or-jsonl-file> [max-unknown-pct]
#   max-unknown-pct defaults to 2 (the D3 bar); 0.5 works for cleanup loops
#   exit: 0 pass · 1 health FAIL · 2 usage/parse · 4 coverage gap
set -euo pipefail

sf_stable=0
manifest=""
expect_skip=0
while [ $# -gt 0 ]; do
	case $1 in
	--servfail-stable)
		sf_stable=1
		shift
		;;
	--manifest)
		[ $# -ge 2 ] || {
			echo "census-gate: --manifest needs a file" >&2
			exit 2
		}
		manifest=$2
		shift 2
		;;
	--expect-skip)
		[ $# -ge 2 ] || {
			echo "census-gate: --expect-skip needs a count" >&2
			exit 2
		}
		expect_skip=$2
		shift 2
		;;
	*) break ;;
	esac
done

if [ $# -lt 1 ]; then
	echo "usage: $0 [--servfail-stable] [--manifest FILE] [--expect-skip N] <stderr-or-jsonl-file> [max-unknown-pct]" >&2
	exit 2
fi

file=$1
budget=${2:-2}

if [ ! -f "$file" ]; then
	echo "census-gate: no such file: $file" >&2
	exit 2
fi

pass_fail() {
	local total=$1 nxdomain=$2 servfail=$3 unknown=$4
	awk -v total="$total" -v nx="$nxdomain" -v sf="$servfail" -v unk="$unknown" -v budget="$budget" -v sf_stable="$sf_stable" '
        BEGIN {
            if (total <= 0) { print "census-gate: FAIL (no domains counted)"; exit 1 }
            rate = unk * 100.0 / total
            if (sf_stable)
                printf "domains %d | nxdomain %d (%.2f%%, domain truth) | servfail-stable %d (%.2f%%, broken delegation, explained) | unknown %d (%.2f%%, budget %.2f%%)\n", total, nx, nx*100.0/total, sf, sf*100.0/total, unk, rate, budget
            else
                printf "domains %d | nxdomain %d (%.2f%%, domain truth) | unknown %d (%.2f%%, budget %.2f%%)\n", total, nx, nx*100.0/total, unk, rate, budget
            if (rate >= budget) { print "census-gate: FAIL — resolver/tool health below bar, do NOT scale"; exit 1 }
            print "census-gate: PASS"
        }'
}

coverage_check() {
	[ -n "$manifest" ] || return 0
	if [ ! -f "$manifest" ]; then
		echo "census-gate: no such manifest: $manifest" >&2
		exit 2
	fi
	local manifest_unique unparseable accounted gap
	# Normalize manifest lines the way domain.Parse does (lowercase, no
	# trailing dot, no leading www.) before uniquing: a manifest carrying
	# both www.foo.ch and foo.ch names ONE scan target, exactly like the
	# feeder's dedup sees it (the www.<public-suffix> policy, 2026-11-12:
	# normalize and scan — the stripped apex has real DNS evidence).
	manifest_unique=$(grep -v '^[[:space:]]*$' "$manifest" | tr 'A-Z' 'a-z' | sed -E 's/\.$//; s/^www\.//' | sort -u | wc -l)
	unparseable=$(sed -nE 's/^skipped ([0-9]+) unparseable.*/\1/p' "$file" | tail -1)
	unparseable=${unparseable:-0}
	# The scan's own accounting lines (2026-11-12): the summary counts only
	# real scans, so interrupted and resume-skipped domains surface here.
	# They explain a gap; they never excuse one (the journal reconciles).
	local interrupted resumed
	interrupted=$(sed -nE 's/^interrupted: ([0-9]+) input domains.*/\1/p' "$file" | tail -1)
	interrupted=${interrupted:-0}
	resumed=$(sed -nE 's/^resume: skipped ([0-9]+) input domains.*/\1/p' "$file" | tail -1)
	resumed=${resumed:-0}
	accounted=$((total + unparseable + expect_skip))
	if [ "$accounted" -ne "$manifest_unique" ]; then
		gap=$((manifest_unique - accounted))
		echo "census-gate: FAIL — coverage gap: manifest $manifest_unique (normalized) vs scanned $total + unparseable $unparseable + expect-skip $expect_skip = $accounted (delta $gap)"
		if [ "$interrupted" -gt 0 ]; then
			echo "census-gate: run was interrupted ($interrupted never scanned) — retry the out-dir (explicit --resume via run-id)"
		fi
		if [ "$resumed" -gt 0 ]; then
			echo "census-gate: run resumed and skipped $resumed observed domains"
		fi
		echo "census-gate: reconcile against the journal (domain_observed events by run_idx) before trusting this pass; see docs/census.md 'resume hazard'"
		exit 4
	fi
	echo "census-gate: coverage PASS — manifest $manifest_unique (normalized) = scanned $total + unparseable $unparseable + expect-skip $expect_skip"
}

summary_pattern='^[0-9]+ domains: workspace certain [0-9]+ / likely [0-9]+, gcp [0-9]+, errors [0-9]+'

if grep -Eq "$summary_pattern" "$file"; then
	# stderr run-summary form:
	# "N domains: workspace certain X / likely Y, gcp Z, errors E (nxdomain a, timeout b, servfail c, canceled d, other e)"
	# A zero-error run prints no kind suffix at all — then every kind is 0.
	line="$(grep -E "$summary_pattern" "$file" | tail -1)"
	nums=$(printf '%s\n' "$line" | sed -nE 's/^([0-9]+) domains:.*errors [0-9]+ \(nxdomain ([0-9]+), timeout ([0-9]+), servfail ([0-9]+), canceled ([0-9]+), other ([0-9]+)\).*/\1 \2 \3 \4 \5 \6/p')
	if [ -z "$nums" ]; then
		nums=$(printf '%s\n' "$line" | sed -nE 's/^([0-9]+) domains:.*/\1 0 0 0 0 0/p')
	fi
	read -r total nx timeouts servfail canceled other <<<"$nums"
	if [ -z "${total:-}" ]; then
		echo "census-gate: found summary line but could not parse it: $line" >&2
		exit 2
	fi
	if [ "$sf_stable" = 1 ]; then unknown=$((timeouts + canceled + other)); else unknown=$((timeouts + servfail + canceled + other)); fi
	pass_fail "$total" "$nx" "$servfail" "$unknown"
	coverage_check
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
	if [ "$sf_stable" = 1 ]; then unknown=$((timeouts + canceled + other)); else unknown=$((timeouts + servfail + canceled + other)); fi
	pass_fail "$total" "$nx" "$servfail" "$unknown"
	coverage_check
else
	echo "census-gate: $file is neither a run summary (stderr) nor jsonl output" >&2
	exit 2
fi
