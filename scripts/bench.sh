#!/usr/bin/env bash
# bench.sh - measure tensorwatch overhead under controlled CPU load.
#
# usage: scripts/bench.sh [interval] [duration_s] [load_pct]
#   interval     sampling period (default 1s)
#   duration_s   how long the run lasts (default 60)
#   load_pct     base CPU load injected by twload (default 30)

set -eu

INTERVAL="${1:-1s}"
DURATION="${2:-60}"
LOAD="${3:-30}"

cd "$(dirname "$0")/.."
make build twload >/dev/null

mkdir -p bench
LOG="bench/run-$(date +%Y%m%dT%H%M%S).log"

echo "tensorwatch bench: interval=$INTERVAL duration=${DURATION}s load=${LOAD}%"
echo "log: $LOG"

./bin/twload -pattern constant -base "$LOAD" -duration "${DURATION}s" >/dev/null &
LOAD_PID=$!

./bin/tensorwatch -headless -http :19199 -interval "$INTERVAL" >"$LOG" 2>&1 &
TW_PID=$!

trap 'kill "$LOAD_PID" "$TW_PID" 2>/dev/null || true' EXIT

sleep 2
SAMPLES=$((DURATION / 2))
ps -o pid,%cpu,rss,command -p "$TW_PID" >>"$LOG"
i=0
while [ "$i" -lt "$SAMPLES" ]; do
  ps -p "$TW_PID" -o %cpu=,rss= 2>/dev/null || break
  sleep 2
  i=$((i + 1))
done | awk '
  { cpu+=$1; rss+=$2; n++ }
  END {
    if (n > 0) {
      printf "samples=%d  avg_cpu=%.2f%%  avg_rss=%.1f MiB\n", n, cpu/n, (rss/n)/1024
    } else {
      print "no samples collected"
    }
  }
' | tee -a "$LOG"
