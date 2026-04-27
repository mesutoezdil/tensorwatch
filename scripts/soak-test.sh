#!/usr/bin/env bash
# soak-test.sh - long-running sanity check.
#
# Runs tensorwatch for hours alongside twload sweeps, polling the HTTP
# API every minute and asserting the process is still alive, RSS has not
# grown past a multiple of its starting value, and /metrics still parses.
#
# usage: scripts/soak-test.sh [hours]

set -eu

HOURS="${1:-4}"
END=$(( $(date +%s) + HOURS * 3600 ))
PORT=19200

cd "$(dirname "$0")/.."
make build twload >/dev/null

./bin/twload -pattern sine -base 30 -amplitude 50 -period 5m >/dev/null &
LOAD_PID=$!

./bin/tensorwatch -headless -http ":$PORT" -interval 1s >/tmp/tw-soak.log 2>&1 &
TW_PID=$!

trap 'kill "$LOAD_PID" "$TW_PID" 2>/dev/null || true' EXIT
sleep 3

START_RSS=$(ps -p "$TW_PID" -o rss= 2>/dev/null | tr -d ' ')
echo "starting RSS: ${START_RSS} KiB  duration: ${HOURS}h"

while [ "$(date +%s)" -lt "$END" ]; do
  if ! kill -0 "$TW_PID" 2>/dev/null; then
    echo "FAIL: tensorwatch process died" >&2
    exit 1
  fi
  if ! curl -fsS --max-time 5 "http://localhost:$PORT/health" >/dev/null; then
    echo "FAIL: /health did not respond" >&2
    exit 1
  fi
  if ! curl -fsS --max-time 5 "http://localhost:$PORT/metrics" | grep -q '^tw_uptime_seconds'; then
    echo "FAIL: /metrics did not contain expected metric" >&2
    exit 1
  fi
  CUR_RSS=$(ps -p "$TW_PID" -o rss= | tr -d ' ')
  if [ "$CUR_RSS" -gt $((START_RSS * 4)) ]; then
    echo "FAIL: RSS grew from ${START_RSS} to ${CUR_RSS} KiB" >&2
    exit 1
  fi
  printf '[%s] alive  rss=%s KiB\n' "$(date +%H:%M:%S)" "$CUR_RSS"
  sleep 60
done

echo "PASS: ${HOURS}h soak completed without regression"
