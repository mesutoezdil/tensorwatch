# Contributing

Thanks for taking a look. tensorwatch is small on purpose, so most contributions land quickly. This page lists the few rules that keep the project easy to maintain.

## Before you start

For anything bigger than a typo or an obvious bug fix, open an issue first and describe the change you want to make. New collectors, new exporters and TUI changes have a few invariants that are easier to discuss in a thread than to debate in a pull request.

## Local setup

Requirements:

- Go 1.23 or newer
- gofmt and go vet (bundled with the toolchain)
- Optional: an NVIDIA Linux box with NVML for `-tags nvidia` builds

```sh
git clone https://github.com/mesutoezdil/tensorwatch
cd tensorwatch
git config core.hooksPath .githooks    # one-time, runs gofmt + vet on commit
make                                    # builds bin/tensorwatch and bin/twload
go test ./...
```

## Project layout

| Path | Purpose |
|------|---------|
| `cmd/tensorwatch` | CLI entrypoint, flag parsing, wiring |
| `cmd/twload` | Synthetic load generator |
| `internal/model` | Snapshot data structures shared by every package |
| `internal/collector` | Collector interface and per-subsystem implementations |
| `internal/pipeline` | Sampling loop and fan-out |
| `internal/peaks` | Windowed peak tracker (decorator) |
| `internal/exporter` | Prometheus exposition and CSV sink |
| `internal/httpapi` | HTTP server for `/snapshot`, `/metrics`, `/health` |
| `internal/tui` | tcell terminal UI |
| `internal/alert` | Threshold engine with sustain windows and webhook delivery |
| `internal/config` | YAML loader |
| `scripts/` | bench and soak helpers |

The package layout is intentionally flat. Resist the urge to add a `pkg/` umbrella, a plugin registry or a DI container.

## Adding a collector

1. Create a new package under `internal/collector/<name>/`.
2. Implement the `Collector` interface from `internal/collector/collector.go`. Write into the shared `*model.Snapshot`.
3. Register it in `cmd/tensorwatch/main.go` next to the existing collectors.
4. If the collector adds new fields to `model.Snapshot`, expose them in the Prometheus exporter (`internal/exporter/prometheus.go`) and the JSON snapshot will follow automatically.

A collector should:

- Read what it needs and return quickly. The pipeline is single-producer, so a slow collector blocks the next tick.
- Never block on the network. If you must, push the work into a goroutine the collector owns and return cached values.
- Tolerate missing data. Set fields to zero rather than returning errors when the host simply does not expose a counter.

## Adding an exporter or sink

Implement a `Consume(ctx, <-chan model.Snapshot)` method. Subscribe via `pipeline.Subscribe(buf)` from main. Subscriber channels are non-blocking, so a stalled exporter drops samples instead of stalling the producer.

## Style

- gofmt is enforced by the pre-commit hook. The repo uses no other linter.
- No comments that restate what the code already says. Save comments for the why behind a non-obvious decision.
- One concern per package. If a function does two things, split it before merging.
- ASCII only in source and docs. The TUI uses Unicode block-drawing glyphs deliberately, but prose stays plain.

## Pull requests

- Branch from `main`. Rebase, do not merge `main` back into your branch.
- Keep the diff focused. A bug fix and a refactor go in two PRs.
- Run `go test ./...`, `go vet ./...` and `make build twload` locally. CI runs the same.
- Title in the imperative ("add ROCm collector", not "added ROCm collector").
- A PR description should answer: what changed, why, how was it tested.

## Reporting issues

Useful issue reports include:

- Output of `tensorwatch -version`
- Host platform (`uname -a` on Linux/macOS)
- Output of `curl -s localhost:9123/snapshot` if the bug is in collected data
- The command line you ran
- What you expected and what happened instead

For TUI rendering bugs, the terminal program and font matter. Please include them.

## Releases

Maintainers tag a release with `git tag vX.Y.Z && git push --tags`. The release workflow builds linux+darwin x amd64+arm64 binaries for both `tensorwatch` and `twload` and publishes them with `SHA256SUMS`. Contributors do not need to do anything for a release.

## License

By contributing you agree that your changes are licensed under the MIT License (see [LICENSE](LICENSE)).
