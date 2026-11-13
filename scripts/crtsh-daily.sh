#!/usr/bin/env bash
# crt.sh daily harvest: rate-limit-aware wrapper around crtsh-extract.sh.
#
# crt.sh allows at most one pull per day per TLD; this wrapper refuses to
# pull a TLD whose newest artifact is younger than 24 h (the guard, not
# the timer cadence, is the contract), keeps the newest KEEP_COUNT
# artifacts per TLD, and logs to monitor.log next to them.
#
# usage: crtsh-daily.sh [tld ...]   (default: de)
# env: GDD_ZONE_DATA_DIR (default reports/census/zone-data)
set -euo pipefail

here=$(cd "$(dirname "$0")" && pwd)
data_dir=${GDD_ZONE_DATA_DIR:-"$here/../../reports/census/zone-data"}
keep_count=${KEEP_COUNT:-3}
min_age_hours=24

mkdir -p "$data_dir"

log() { printf '%s %s\n' "$(date '+%F %T')" "$*" >>"$data_dir/monitor.log"; }

pull_tld() {
	local tld=$1 newest age_hours outfile
	newest=$(ls -t "$data_dir/$tld"-crtsh-*.txt 2>/dev/null | head -1 || true)

	if [ -n "$newest" ]; then
		age_hours=$((($(date +%s) - $(stat -c %Y "$newest")) / 3600))
		if [ "$age_hours" -lt "$min_age_hours" ]; then
			log "$tld: skip — newest artifact $newest is ${age_hours}h old (< ${min_age_hours}h rate limit)"
			return 0
		fi
	fi

	outfile="$data_dir/$tld-crtsh-$(date '+%Y%m%d').txt"
	log "$tld: pulling cert-transparency domains → $outfile (this can take minutes; crt.sh is slow)"

	if "$here/crtsh-extract.sh" "$tld" "$outfile"; then
		log "$tld: $(wc -l <"$outfile") registrable domains harvested"
	else
		log "$tld: FAILED (see above); artifact removed if partial"
		rm -f "$outfile"
		return 1
	fi

	ls -t "$data_dir/$tld"-crtsh-*.txt 2>/dev/null | tail -n +"$((keep_count + 1))" |
		while IFS= read -r stale; do
			rm -f "$stale"
			log "$tld: rotated out $stale"
		done

	return 0
}

tlds=${*:-de}
rc=0

for tld in $tlds; do
	pull_tld "$tld" || rc=1
done

exit "$rc"
