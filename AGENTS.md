# AGENTS.md

Guidance for AI agents working in this repository.

## Project Overview

**gtexporter** (`github.com/automixer/gtexporter`) is a vendor-agnostic Prometheus exporter
that subscribes to gNMI telemetry streams from network devices and converts them into
Prometheus metrics using YANG data models (yGot-generated Go structs). Multiple devices
are served concurrently: one `GnmiClient` per device, each running multiple schema plugins
(one per group of YANG paths).

Go 1.26. Entry point: `cmd/gtexporter/main.go` (requires `-config <file.yaml>`; `-version` prints build info).

## Commands

```bash
make              # default target is `devel`: fmt + vet + build to build/gtexporter (dev-<commit> version)
make vet          # go fmt + go vet
make lint         # golangci-lint run (config: .golangci.yml)
make release      # build with version from git tags -> build/gtexporter-<os>-<arch>
make docker_release  # release versioning, plain binary name (used by Dockerfile)
```

**There are no tests in this repo and none are planned.** Verification = `make` (fmt+vet)
plus `make lint`. Do not add a test suite unprompted.

Never run `make gen_ysocif`, `make gen_ysoclldp`, or `make install_ygot_gen`. Those targets
regenerate the yGot datamodels and are for human maintainers only.

## Architecture

```
cmd/gtexporter/main.go
  -> pkg/core          Core: parses YAML config (confmgmt.go), wires everything (core.go)
    -> pkg/exporter    Prometheus HTTP server + Collector; Registry() is a package-level registry
    -> pkg/gnmiclient  One per device: gRPC dial, capability check, Subscribe RPC loop
      -> pkg/plugins   One per (device, plugin): Parser + Formatter pair
```

Data flow:

1. `GnmiClient` subscribes to the union of all its plugins' XPaths and loops receiving
   `gnmi.Notification` messages.
2. Notifications are routed to plugins by path prefix. Routing strips YANG list keys
   (`[name=...]`) before matching (`gnmiclient.go` `RegisterPlugin`), so two plugins can
   never claim the same key-stripped path.
3. In **passthrough mode** (default) the plugin buffers raw notifications in a `uBuffer`
   and only parses them at scrape time, then clears the parsed state. In **cache mode**
   (`mode: cache`) the parser keeps the GoStruct persistently across scrapes and depends
   on gNMI DELETE messages to evict stale data.
4. On each Prometheus scrape, `Exporter.Collect()` fans out to every registered
   `GMetricSource`; each `Plugin.GetMetrics()` runs `Formatter.ScrapeEvent(GoStruct)`
   (which returns a cleanup func, deferred) then `Formatter.Collect()`.

## Adding a Plugin

Follow `pkg/plugins/ocinterfaces/` or `pkg/plugins/oclldp/`:

1. Create `pkg/plugins/<name>/` with a Parser (decodes `gnmi.Notification` into a yGot
   GoStruct) and a Formatter (reads the GoStruct and emits metrics). Interfaces are in
   `pkg/plugins/plugin.go`.
2. Provide `newParser(cfg plugins.Config) (plugins.Parser, error)` and
   `newFormatter(cfg plugins.Config) (plugins.Formatter, error)`.
3. Call `plugins.Register(plugName, newFormatter, newParser)` from `init()`
   (note the argument order: formatter first).
4. Blank-import the package in `pkg/core/core.go` — forgetting this silently makes the
   plugin name unknown at config load time.
5. Per-plugin options arrive as `map[string]string` in `cfg.Options`; parse with
   `strconv.ParseBool` etc. (see `ocifformatter.go:47`). Document them in `plugin-options.yaml`.

## Metric System

- Metrics are plain structs embedding `exporter.MetricCommons` (Name, Help, Device, Type,
  Value). Any string field tagged `label:"<key>"` becomes a Prometheus label, extracted by
  reflection at collection time (`pkg/exporter/gmetric.go`). Label fields must be strings.
- FQName is built as `<metric_prefix>_<name>_total` for counters and
  `<metric_prefix>_<name>_gauges` for gauges (`buildFQName`). Note the unusual `_gauges`
  suffix for gauges — match existing behavior, don't "fix" it.
- `GnmiClient` and each `Plugin` also emit self-monitoring metrics (connection errors,
  parse failures, `collected_series`) through the same `GMetricSource` interface; these
  are registered unconditionally.

## Configuration

- YAML, parsed with `yaml.UnmarshalStrict` — unknown keys are fatal errors. All valid
  keys are documented in `config-keys.yaml`; keep that file in sync when adding keys.
- Three levels: `global` -> `device_template` (defaults merged into each device) ->
  per-device overrides. Merge logic lives in `pkg/core/confmgmt.go`.
- Config semantics worth knowing:
  - `scrape_interval` must equal the Prometheus server scrape interval; gNMI
    `sample_interval = scrape_interval / oversampling`.
  - `mode: cache` requires the device to send gNMI DELETEs; `on_change: true` is only
    compatible with cache mode.
  - `max_life` forces periodic reconnect + cache flush (workaround for devices without
    DELETE support).

## Conventions and Gotchas

- Logging is `log "github.com/golang/glog"`, not a custom logging package.
- `pkg/datamodels/**/gen.go` is yGot-generated code. **Never hand-edit it.**
  `.golangci.yml` excludes all of `pkg/datamodels/` from linting.
- Device credentials come from the YAML config; `rpccreds.go` implements gRPC
  per-RPC credentials. Never log passwords.
- `Core.Run` starts the exporter first, then devices; on shutdown the order is reversed
  (exporter closes before clients). Client start errors are logged but not fatal.
- Existing style: `any` (not `interface{}`), `for i := range n` for simple index loops,
  `slices.Contains`/`slices.Max`/`slices.Min` over manual loops, `errors.Is`/`errors.As`
  for error checks, table-driven small helpers, minimal deps (go.mod is short — prefer
  stdlib over new dependencies).
- CI (`.github/workflows/`): `lint.yaml` runs golangci-lint, `build_branch.yaml` builds
  branches, `release.yaml` cuts releases from tags.
