#!/usr/bin/env bash
# Lifecycle manager for the census resolver (user-scoped unbound).
# Usage: scripts/unbound.sh {start|stop|status|restart}
#
# Default instance: the canonical UDP forwarder, scripts/unbound.conf,
# on 127.0.0.1:5353. A second instance (e.g. the TCP-only variant,
# scripts/unbound-tcp.conf on :5354) is selected via GDD_UNBOUND_CONF;
# pid/log files derive from the conf basename so instances are
# independently managed (stopping one never kills the other).
#
# Runtime artifacts (pid, log) live under reports/census/unbound/
# (gitignored). Never runs as root; never touches the system resolver.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
CONF="${GDD_UNBOUND_CONF:-$REPO_ROOT/scripts/unbound.conf}"
RUN_DIR="${GDD_CENSUS_DIR:-$REPO_ROOT/reports/census/unbound}"
INSTANCE="$(basename "$CONF" .conf)"
PID_FILE="$RUN_DIR/$INSTANCE.pid"
LOG_FILE="$RUN_DIR/$INSTANCE.log"
LISTEN_ADDR="$(awk '/^[[:space:]]*interface:/ {print $2; exit}' "$CONF")"
LISTEN_PORT="$(awk '/^[[:space:]]*port:/ {print $2; exit}' "$CONF")"

mkdir -p "$RUN_DIR"

if [ ! -f "$CONF" ]; then
	echo "unbound.sh: no such config: $CONF" >&2
	exit 2
fi

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
		echo "$INSTANCE already running (pid $(cat "$PID_FILE"))"
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
		echo "$INSTANCE started (pid $(cat "$PID_FILE")) on $LISTEN_ADDR:$LISTEN_PORT"
	else
		echo "$INSTANCE failed to start; see $LOG_FILE" >&2
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
		echo "$INSTANCE stopped (pid $pid)"
	else
		echo "$INSTANCE not running"
	fi
	rm -f "$PID_FILE"
}

status() {
	if is_running; then
		echo "$INSTANCE running (pid $(cat "$PID_FILE")) on $LISTEN_ADDR:$LISTEN_PORT"
		if dig +time=2 +tries=1 "@$LISTEN_ADDR" -p "$LISTEN_PORT" ch. SOA +short >/dev/null 2>&1; then
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
	echo "usage: $0 {start|stop|status|restart}  [GDD_UNBOUND_CONF=<conf> for a second instance]" >&2
	exit 2
	;;
esac
