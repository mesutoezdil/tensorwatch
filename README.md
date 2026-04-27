# tensorwatch

A pluggable, terminal-first observability agent for machines that run heavy compute. Written in Go, single binary, no runtime dependencies.

tensorwatch streams CPU, memory and GPU telemetry through a small in-process pipeline and pushes it into the surfaces you actually use: a live TUI, a Prometheus endpoint, a JSON HTTP API, a CSV log, and threshold-based alerts that hit a webhook.

```
┌─ collectors ────────┐    ┌─ pipeline ─┐    ┌─ sinks ────────────────┐
│ host  cpu  mem  gpu │ ─▶ │  fan-out   │ ─▶ │ TUI  HTTP  CSV  alerts │
└─────────────────────┘    └────────────┘    └────────────────────────┘
```

## Features

- TUI rebuilt on tcell with sparkline history, per-core bars, GPU panel
- HTTP API with three endpoints — `/snapshot` (JSON), `/metrics` (Prometheus exposition), `/health`
- Optional Bearer-token auth for the HTTP API (CLI flag, YAML, or `TENSORWATCH_TOKEN` env)
- YAML config with override-by-flag semantics, plus an alert engine with sustain windows
- CSV append-only sink for offline analysis
- NVIDIA GPU support behind a build tag — default builds compile cleanly on any platform with no NVML dependency
- Cross-compiles to linux/amd64, linux/arm64, darwin/amd64, darwin/arm64 (CGO not required for the default build)

## Install

```sh
git clone https://github.com/mesutoezdil/tensorwatch
cd tensorwatch
make build
./bin/tensorwatch
```

For NVIDIA GPU collection on Linux:

```sh
make build-nvidia       # requires CGO + NVIDIA driver / NVML
```

Cross-platform release artifacts:

```sh
make release            # writes dist/tensorwatch-{linux,darwin}-{amd64,arm64}
```

## Usage

```sh
tensorwatch                              # TUI only, 1s interval
tensorwatch -interval 500ms              # faster refresh
tensorwatch -http :9123                  # TUI + HTTP API
tensorwatch -headless -http :9123        # headless exporter
tensorwatch -csv samples.csv             # append metrics to CSV
tensorwatch -config examples/config.yaml # YAML config (flags still override)
```

### Flags

| Flag | Purpose |
|---|---|
| `-config FILE` | YAML config path |
| `-interval D` | Sampling interval (e.g. `1s`, `500ms`) |
| `-http ADDR` | Enable HTTP API on `host:port` |
| `-http-token T` | Require `Authorization: Bearer T` for `/snapshot` and `/metrics` |
| `-csv FILE` | Append samples to CSV file |
| `-headless` | Disable TUI |
| `-version` | Print version |

### Keys

`q` / `Esc` quit. The TUI auto-resizes; metrics keep flowing in the background regardless of focus.

## HTTP API

```sh
curl localhost:9123/health
curl localhost:9123/snapshot      # full JSON snapshot
curl localhost:9123/metrics       # Prometheus exposition
```

Prometheus scrape config:

```yaml
scrape_configs:
  - job_name: tensorwatch
    static_configs:
      - targets: ['host:9123']
    authorization:
      credentials: 'your-token'
```

Metric names follow the `tw_*` prefix: `tw_cpu_usage_percent`, `tw_gpu_utilization_percent`, `tw_memory_used_bytes`, etc.

## Alerts

Alerts evaluate against the latest snapshot. A rule fires only after its threshold has been breached continuously for the configured `sustain` window — this avoids paging on transient spikes.

```yaml
alert_webhook: "https://hooks.example.com/tensorwatch"
alerts:
  - name: gpu_thermal
    metric: gpu.temp
    operator: ">"
    value: 82
    sustain: 30s
```

Available metrics for alert rules: `cpu.overall`, `cpu.temp`, `mem.used_pct`, `load.1`, `gpu.util`, `gpu.temp`, `gpu.mem_used_pct`. The webhook receives a JSON body with the rule, metric, value, and message.

## Architecture

- `internal/collector` — each subsystem implements a tiny `Collector` interface and writes into a shared snapshot. Adding a new source (disks, RDMA, ROCm) is one file.
- `internal/pipeline` — single goroutine ticks collectors at `interval`, fans the snapshot out to subscriber channels with non-blocking sends. Slow consumers can drop samples; they cannot stall the producer.
- `internal/exporter`, `internal/httpapi`, `internal/tui`, `internal/alert` — independent consumers of the snapshot stream. None of them know about each other.
- `internal/config` — YAML with strict duration parsing; CLI flags override YAML.

The package layout is intentionally flat: one concern per package, no framework, no DI container, no plugin registry.

## Platforms

| Target | Default build | With `-tags nvidia` |
|---|---|---|
| Linux amd64/arm64 | host + CPU + memory | + NVIDIA GPUs via NVML |
| macOS amd64/arm64 | host + CPU + memory | n/a |
| Windows amd64 | host + CPU + memory | n/a |

GPU support outside NVIDIA (AMD ROCm, Apple Metal counters) is unimplemented but the collector interface is built for it.

## Status

Alpha. The data model and CLI flags may still shift. Pin a tag if you embed it in tooling.

## License

MIT — see [LICENSE](LICENSE).
