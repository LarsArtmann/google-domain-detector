#!/usr/bin/env bash
# Lifecycle manager for the census resolver (user-scoped unbound on
# 127.0.0.1:5353). Usage: scripts/unbound.sh {start|stop|status|restart}
#
# The canonical config is scripts/unbound.conf (tracked); runtime
# artifacts (unbound.log, unbound.pid) live under reports/census/unbound/
# (gitignored). Never runs as root; never touches the system resolver.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
CONF="$REPO_ROOT/scripts/unbound.conf"
RUN_DIR="${GDD_CENSUS_DIR:-$REPO_ROOT/reports/census/unbound}"
PID_FILE="$RUN_DIR/unbound.pid"
LOG_FILE="$RUN_DIR/unbound.log"

mkdir -p "$RUN_DIR"

resolve_unbound() {
	if command -v unbound >/dev/null 2>&1; then
		command -v unbound
	else
		echo "unbound not on PATH; entering a user-scoped nix shell" >&2
		echo "nix"
	fi
}

is_running() {
	[ -f "$PID_FILE" ] && kill -0 "$(cat "$PID_FILE")" 2>/dev/null
}

start() {
	if is_running; then
		echo "unbound already running (pid $(cat "$PID_FILE"))"
		return 0
	fi
	local bin
	bin="$(resolve_unbound)"
	if [ "$bin" = "nix" ]; then
		# shellcheck disable=SC2086
		nohup nix shell nixpkgs#unbound -c unbound -d -c "$CONF" \
			>>"$LOG_FILE" 2>&1 &
	else
		nohup "$bin" -d -c "$CONF" >>"$LOG_FILE" 2>&1 &
	fi
	echo $! >"$PID_FILE"
	sleep 1
	if is_running; then
		echo "unbound started (pid $(cat "$PID_FILE")) on 127.0.0.1:5353"
	else
		echo "unbound failed to start; see $LOG_FILE" >&2
		return 1
	fi
}

stop() {
	if is_running; then
		local pid
		pid="$(cat "$PID_FILE")"
		kill "$pid"
		for _ in 1 2 3 4 5; do
			kill -0 "$pid" 2>/dev/null || break
			sleep 1
		done
		kill -0 "$pid" 2>/dev/null && kill -9 "$pid" || true
		echo "unbound stopped (pid $pid)"
	else
		echo "unbound not running"
	fi
	rm -f "$PID_FILE"
}

status() {
	if is_running; then
		echo "unbound running (pid $(cat "$PID_FILE")) on 127.0.0.1:5353"
		if dig +time=2 +tries=1 @127.0.0.1 -p 5353 ch. SOA +short >/dev/null 2>&1; then
			echo "health: answering queries"
		else
			echo "health: NOT answering queries" >&2
			return 1
		fi
	else
		echo "unbound not running"
		return 1
	fi
}

case "${1:-}" in
start) start ;;
stop) stop ;;
status) status ;;
restart)
	stop
	start
	;;
*)
	echo "usage: $0 {start|stop|status|restart}" >&2
	exit 2
	;;
esac
