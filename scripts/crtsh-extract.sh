#!/usr/bin/env bash
# crt.sh extractor: pull certificate-log domains for a TLD (default .ch)
# and reduce them to a deduplicated registrable-domain census input.
#
# DENIC refuses all third-party .de zone access (docs/zone-data/denic-de.md),
# so crt.sh certificate transparency is the only .de fallback beyond Tranco;
# for .ch it augments the SWITCH zone pull. crt.sh is rate-limited and slow —
# run it no more than once a day per TLD.
#
# usage: crtsh-extract.sh <tld> [outfile]
#   TLD without dot, e.g. `ch` or `de`. Outfile defaults to stdout.
#   CRTSH_JSON=<file> reads a pre-downloaded JSON dump instead of hitting
#   the API (fixture tests use this).
#   --test runs the bundled fixture self-test and exits.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
FIXTURE="$REPO_ROOT/scripts/testdata/crtsh-fixture.json"
FIXTURE_EXPECTED="$REPO_ROOT/scripts/testdata/crtsh-fixture-expected.txt"

selftest() {
	actual=$(CRTSH_JSON="$FIXTURE" "$0" ch /dev/stdout)
	expected=$(cat "$FIXTURE_EXPECTED")
	if [ "$actual" != "$expected" ]; then
		echo "crtsh-extract self-test FAILED" >&2
		diff <(printf '%s\n' "$expected") <(printf '%s\n' "$actual") >&2 || true
		exit 1
	fi
	echo "crtsh-extract self-test PASS"
	exit 0
}

[ "${1:-}" = "--test" ] && selftest

if [ $# -lt 1 ]; then
	echo "usage: $0 <tld> [outfile]   (--test runs the fixture self-test)" >&2
	exit 2
fi

tld=${1#.}
out=${2:-/dev/stdout}

if [[ ! "$tld" =~ ^[a-z]{2,}$ ]]; then
	echo "crtsh-extract: TLD must be 2+ letters, got: $tld" >&2
	exit 2
fi

if [ -n "${CRTSH_JSON:-}" ]; then
	cat "$CRTSH_JSON"
else
	# --resolve not needed here (this host's resolver handles crt.sh; if it
	# ever does not, download the dump separately and pass CRTSH_JSON=...).
	curl -fsSL --max-time 300 "https://crt.sh/?q=%25.${tld}&output=json"
fi | jq -r '
    # crt.sh rows carry name_value as newline-joined SAN lists and
    # common_name separately; both can carry wildcards and subdomains.
    # (.[] is explicit: some jq builds do not auto-iterate array inputs.)
    .[] | [.common_name, .name_value[]?] | join("\n")
    | split("\n")[]
' | tr '[:upper:]' '[:lower:]' | sed -E '
    s/^\*\.//          # strip leading wildcards
    s/\.$//            # fully-qualified dots
' | awk -v tld="$tld" '
    # Registrable only: exactly the apex form <label>.<tld> (subdomains and
    # deeper names collapse to their apex by dropping leading labels until
    # two remain — crt.sh logs www.foo.ch and foo.ch alike).
    {
        n = split($0, labels, ".")
        if (n < 2) next
        apex = labels[n-1] "." labels[n]
        if (apex !~ ("[^.]+\\." tld "$")) next
        if (labels[n-1] == "" || labels[n] == "") next
        print apex
    }
' | sort -u >"$out"
