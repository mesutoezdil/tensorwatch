# tensorwatch

[![build](https://github.com/mesutoezdil/tensorwatch/actions/workflows/build.yml/badge.svg)](https://github.com/mesutoezdil/tensorwatch/actions/workflows/build.yml)
[![release](https://img.shields.io/github/v/release/mesutoezdil/tensorwatch?display_name=tag&sort=semver)](https://github.com/mesutoezdil/tensorwatch/releases)
[![Go Report Card](https://goreportcard.com/badge/github.com/mesutoezdil/tensorwatch)](https://goreportcard.com/report/github.com/mesutoezdil/tensorwatch)
[![license](https://img.shields.io/github/license/mesutoezdil/tensorwatch)](LICENSE)

A pluggable, terminal-first observability agent for machines that run heavy compute. Single static Go binary, cross-platform, no runtime dependencies.

tensorwatch streams CPU, memory, process and GPU telemetry through a small in-process pipeline and pushes it into the surfaces you actually use: a colored TUI, a Prometheus endpoint, a JSON HTTP API, a CSV log, and threshold-based alerts that hit a webhook.

```
┌─ collectors ───────────────┐    ┌─ pipeline ─┐    ┌─ sinks ────────────────┐
│ host  cpu  mem  proc  gpu  │ ─▶ │  fan-out   │ ─▶ │ TUI  HTTP  CSV  alerts │
└────────────────────────────┘    └────────────┘    └────────────────────────┘
```

## Highlights

- **Color-coded TUI** with adaptive layout, per-core bars, GPU panel, top process table, combined sparkline history at the bottom
- **Windowed peak tracking** (default 30 minutes) for CPU, GPU, memory and temperature — peaks render alongside the live value
- **HTTP API** with three endpoints: `/snapshot` (full JSON), `/metrics` (Prometheus), `/health`
- **YAML config** with flag overrides; alert engine fires only after a threshold has been breached for a sustained window
- **CSV append-only sink** for offline analysis
- **NVIDIA GPU support** via NVML, gated behind a build tag — the default build needs no NVIDIA libraries and runs anywhere
- **Cross-compiles** to linux/amd64, linux/arm64, darwin/amd64, darwin/arm64 (CGO not required for the default build)
- **`twload`** companion tool generates synthetic CPU load (sine, step, spike, constant) for end-to-end pipeline validation

## Install

### Pre-built binaries

Each release publishes static binaries for Linux and macOS, both amd64 and arm64, plus `SHA256SUMS`:

```
https://github.com/mesutoezdil/tensorwatch/releases/latest
```

### From source

```sh
git clone https://github.com/mesutoezdil/tensorwatch
cd tensorwatch
make             # builds bin/tensorwatch and bin/twload
./bin/tensorwatch
```

For NVIDIA GPU collection on Linux:

```sh
make build-nvidia    # requires CGO + NVIDIA driver / NVML
```

System-wide install:

```sh
sudo make install    # /usr/local/bin/tensorwatch and /usr/local/bin/twload
```

## Usage

```sh
tensorwatch                              # interactive TUI, 1s refresh
tensorwatch -interval 500ms              # half-second refresh
tensorwatch -compact                     # smaller layout, no process table
tensorwatch -http :9123                  # TUI + Prometheus / JSON HTTP API
tensorwatch -headless -http :9123        # exporter only
tensorwatch -csv samples.csv             # append metrics to CSV
tensorwatch -config examples/config.yaml # YAML config (flags still override)
```

### Flags

| Flag | Description | Default |
|------|-------------|---------|
| `-config FILE` | YAML config path | none |
| `-interval D` | Sampling interval (`500ms`, `1s`, …) | `1s` |
| `-http ADDR` | Enable HTTP API on `host:port` | off |
| `-http-token T` | Require `Authorization: Bearer T` | off |
| `-csv FILE` | Append samples to CSV file | off |
| `-headless` | Disable TUI | off |
| `-compact` | Compact TUI without process table | off |
| `-top N` | Number of processes shown in TUI | 8 |
| `-peak-window D` | Rolling window for peak tracking | 30m |
| `-version` | Print version | |

### Interactive keys

| Key | Action |
|-----|--------|
| `q` / `Esc` / `Ctrl+C` | Quit |
| `c` | Toggle compact layout |

The TUI auto-resizes; metrics keep flowing in the background regardless of focus.

## TUI layout

```
 tensorwatch v0.2.0  myhost  up 3h20m            load 1.20 / 1.45 / 0.80    2026-04-27 12:00:00
 ──────────────────────────────────────────────────────────────────────────────────────────────
 CPU                                                  GPU
 Apple M4 Pro · 14L/14P · 39.3°C                      [0] NVIDIA GeForce RTX 4090
                                                      util  [██████████░░░░░░░░░]   54.0%  pk  92%
 overall [██████░░░░░░░░░░░░]   32.4%   pk  88%       vram  [██████░░░░░░░░░░░░░]   31.0%
                                                      mem 7.4 GiB / 24.0 GiB  pwr 280W  tmp 67°C
   0 [█████░░░░░░] 22.0%     7 [█░░░░░░░░░░] 4.0%     clk gfx 2520 MHz · mem 10501 MHz  fan 38%
   1 [██████░░░░░] 28.0%     8 [░░░░░░░░░░░] 0.0%
   ...

 PROCESSES
   PID  USER             CPU%   MEM%       RSS  COMMAND
  1234  mesut            45.2    8.1   1.2 GiB  go
   822  root              7.3    1.4 220.4 MiB  systemd-resolve
   ...

 MEMORY
 ram  [█████████░░░░░░░]   64.5%   10.0 GiB / 16.0 GiB   pk  78%
 swap [░░░░░░░░░░░░░░░░]    0.0%        0 B  / 0 B
 HISTORY
 cpu  ▁▁▂▃▄▅▆▆▆▅▄▃▂▁▁▁▂▃▃▃▂▁▁▁
 gpu  ▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁
 mem  ▆▆▆▆▆▆▆▆▆▆▆▆▆▆▆▆▆▆▆▆▆▆▆▆
                                                          q quit · c compact · pk = 30m peak
```

Bars are colored: green under 60%, yellow 60–90%, red above 90%. The bottom history charts render the last 240 samples per signal.

> Screenshots live in [`screenshots/`](screenshots/) — replace the placeholders with captures from your own host.

## HTTP API

```sh
curl localhost:9123/health          # liveness
curl localhost:9123/snapshot        # full JSON snapshot (cpu, mem, gpus, processes, peaks)
curl localhost:9123/metrics         # Prometheus exposition
```

### Prometheus scrape config

```yaml
scrape_configs:
  - job_name: tensorwatch
    static_configs:
      - targets: ['host:9123']
    authorization:
      credentials: 'your-token'
```

### Available metrics

| Metric | Type | Labels |
|--------|------|--------|
| `tw_uptime_seconds` | gauge | – |
| `tw_load_average` | gauge | `window` (1m/5m/15m) |
| `tw_cpu_usage_overall_percent` | gauge | – |
| `tw_cpu_usage_percent` | gauge | `core` |
| `tw_cpu_temperature_celsius` | gauge | – |
| `tw_cpu_frequency_mhz` | gauge | – |
| `tw_memory_total_bytes` | gauge | – |
| `tw_memory_used_bytes` | gauge | – |
| `tw_memory_available_bytes` | gauge | – |
| `tw_memory_bufcache_bytes` | gauge | – |
| `tw_swap_total_bytes` | gauge | – |
| `tw_swap_used_bytes` | gauge | – |
| `tw_gpu_utilization_percent` | gauge | `gpu`, `vendor`, `name` |
| `tw_gpu_memory_utilization_percent` | gauge | `gpu`, `vendor`, `name` |
| `tw_gpu_memory_total_bytes` | gauge | `gpu`, `vendor`, `name` |
| `tw_gpu_memory_used_bytes` | gauge | `gpu`, `vendor`, `name` |
| `tw_gpu_temperature_celsius` | gauge | `gpu`, `vendor`, `name` |
| `tw_gpu_power_watts` | gauge | `gpu`, `vendor`, `name` |
| `tw_gpu_power_limit_watts` | gauge | `gpu`, `vendor`, `name` |
| `tw_gpu_clock_mhz` | gauge | `gpu`, `vendor`, `name`, `domain` |
| `tw_gpu_fan_percent` | gauge | `gpu`, `vendor`, `name` |

### Authentication

Bearer token auth is optional but supported via flag, YAML, or environment:

```sh
tensorwatch -http :9123 -http-token "$(cat /etc/tensorwatch.token)"
TENSORWATCH_TOKEN=$(cat /etc/tensorwatch.token) tensorwatch -http :9123
```

The env var avoids exposing the secret in `ps` output and is preferred. Without a token, the endpoint is open — combine with Tailscale, an SSH tunnel, or a reverse proxy for transport security.

## Alerts

Alerts evaluate against the latest snapshot. A rule fires only after its threshold has been breached continuously for the configured `sustain` window — no paging on transient spikes.

```yaml
alert_webhook: "https://hooks.example.com/tensorwatch"

alerts:
  - name: gpu_thermal
    metric: gpu.temp
    operator: ">"
    value: 82
    sustain: 30s

  - name: memory_pressure
    metric: mem.used_pct
    operator: ">"
    value: 92
    sustain: 1m
```

Available rule metrics: `cpu.overall`, `cpu.temp`, `mem.used_pct`, `load.1`, `gpu.util`, `gpu.temp`, `gpu.mem_used_pct`. The webhook receives a JSON body containing the rule, metric, value, threshold, operator, message and timestamp.

## Synthetic load

`twload` drives the host with controlled CPU load — useful when verifying a fresh deployment, validating Grafana dashboards, or smoke-testing alert rules without waiting for real traffic.

```sh
twload                                  # 50% sine wave on every core, 30s period
twload -workers 4 -pattern step         # square wave, 4 cores
twload -base 80 -amplitude 20 -period 5s -duration 2m
twload -pattern spike -base 5 -amplitude 90 -period 10s
```

Patterns: `sine` (smooth sweep), `step` (square wave), `spike` (short bursts), `constant` (flat).

## Architecture

- **`internal/collector`** — each subsystem (`host`, `cpu`, `memory`, `processes`, `gpu`) implements the same tiny `Collector` interface and writes into a shared snapshot. Adding a new source (disk, RDMA, ROCm, Apple Metal counters) is one file with no central registration.
- **`internal/pipeline`** — a single goroutine ticks collectors at `interval`, applies decorators (peak tracker), and fans the snapshot out to subscriber channels with non-blocking sends. Slow consumers can drop samples; they cannot stall the producer.
- **`internal/peaks`** — keeps a rolling window of samples and decorates each snapshot with windowed peak values before fan-out.
- **`internal/exporter`, `internal/httpapi`, `internal/tui`, `internal/alert`** — independent consumers of the snapshot stream. None of them know about each other.
- **`internal/config`** — YAML with strict duration parsing; CLI flags override YAML.

The package layout is intentionally flat: one concern per package, no framework, no DI container, no plugin registry.

## Platforms

| Target | Default build | With `-tags nvidia` |
|--------|--------------|---------------------|
| Linux amd64/arm64 | host + CPU + memory + processes | + NVIDIA GPUs via NVML |
| macOS amd64/arm64 | host + CPU + memory + processes | n/a |
| Windows amd64 | host + CPU + memory + processes | n/a |

GPU support beyond NVIDIA (AMD ROCm, Apple Metal counters) is unimplemented but the collector interface is built for it — see `internal/collector/gpu/stub.go` for the contract.

## Performance

tensorwatch is designed to stay invisible on the host it monitors. See [PERFORMANCE.md](PERFORMANCE.md) for measurement methodology and overhead figures.

## Status

Alpha. The data model and CLI flags may still shift between minor versions. Pin a tag if you embed it in tooling.

## License

MIT — see [LICENSE](LICENSE).
