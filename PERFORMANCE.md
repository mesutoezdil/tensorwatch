# Performance notes

tensorwatch is designed to be a low-overhead resident on hosts that already have something more important to do. This page tracks what that means in practice.

## Footprint

- Single static binary, ~7 MB stripped (default build, no NVML).
- Resident memory under typical load (1 s interval, 14 cores, no GPU): ~12 MiB RSS.
- Static binary - no shared library load cost, no Python interpreter, no JVM.

## Sampling cost

The pipeline runs in one goroutine and ticks at the configured interval. Each tick:

1. Calls `Collect` on each registered collector. Most collectors read from `/proc`, `/sys` or platform syscalls. None open new connections, none cache file descriptors beyond the lifetime of the process.
2. Applies decorators (peak tracker today, future hooks plug in here).
3. Fans the resulting snapshot out to subscriber channels via non-blocking sends.

Slow subscribers (a saturated webhook, a stalled HTTP client) drop samples but cannot stall the producer.

## Measuring overhead on your host

Build the binary, run it under `time` and `pidstat` while a representative workload runs alongside:

```sh
make build
./bin/tensorwatch -headless -http :9123 -interval 1s &
TWPID=$!
pidstat -p "$TWPID" -u -r 5 12      # 12 samples, 5s apart
kill "$TWPID"
```

Record the average `%CPU` and `RSS`. Repeat at the interval you intend to deploy with. Most fleets are fine at 1 s. If you push below 250 ms you will start to see the sample collection cost dominate.

## Synthetic stress

`twload` provides reproducible CPU load for measuring tensorwatch under stress and for validating dashboards and alert rules:

```sh
./bin/twload -pattern sine -base 50 -amplitude 50 -period 30s
```

Run tensorwatch alongside it and confirm the sparklines and Prometheus values track the load curve.

## Known limits

- Per-process CPU percentages on macOS are reported by `gopsutil` against a single-core ceiling. On Linux they're reported per-core. The TUI shows the raw value, so divide by core count if you want a host-wide ratio on Linux.
- CPU frequency on Apple Silicon is not exposed through the same path as on Linux. The TUI suppresses unreliable values and the Prometheus exporter does the same.
- Process collection is enabled only when the TUI is active and not in compact mode. Enabling it for headless exporters will be a config option in a future release. Until then keep `-headless` for pure exporters.
