# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

**gtexporter** is a vendor-agnostic Prometheus exporter that subscribes to gNMI (gRPC Network Management Interface) telemetry streams from network devices and exports metrics using YANG data models. It supports horizontal scaling: multiple instances run concurrently, each managing multiple gNMI clients (one per device), each client running multiple schema plugins (one per group of YANG paths).

Module: `github.com/automixer/gtexporter`

## Commands

```bash
# Build development binary (runs fmt + vet first)
make

# Format and vet only
make vet

# Build release binary (uses git tags for version)
make release

# Build Docker image locally
docker build --build-arg BUILD_MODE=devel -t gtexporter .

# Run the binary
./build/gtexporter -config <path/to/config.yaml>

# Generate YANG GoStructs (requires make install_ygot_gen first)
make install_ygot_gen
make gen_ysocif      # OpenConfig interfaces
make gen_ysoclldp    # OpenConfig LLDP
```

There are no automated tests in this repository and are not expected to be.

## Architecture

### Component Hierarchy

```
Core (pkg/core/)
 └── Exporter (pkg/exporter/)         ← Prometheus HTTP server + Collector
 └── GnmiClient (pkg/gnmiclient/)     ← one per device, manages gNMI session
      └── Plugin (pkg/plugins/)        ← one per YANG plugin per device
           ├── Parser                  ← decodes gNMI Notifications → yGot GoStruct
           └── Formatter               ← reads GoStruct → Prometheus metrics
```

### Data Flow

1. `GnmiClient` dials the device, checks capabilities, opens a Subscribe RPC, then loops receiving `gNMI.Notification` messages.
2. Each notification is routed by path prefix to the matching `Plugin.Notification()`.
3. In **cache mode** the Parser stores the GoStruct persistently (requires gNMI DELETE handling). In **passthrough mode** notifications are buffered in `ubuffer` and parsed only at scrape time.
4. On each Prometheus scrape, `Exporter.Collect()` fans out to all registered `GMetricSource` instances, each `Plugin.GetMetrics()` calls `Formatter.ScrapeEvent(GoStruct)` then `Collect()` to emit Prometheus metrics.

### Plugin System

Plugins are registered via a factory pattern in `pkg/plugins/register.go`. Each plugin's `init()` calls `plugins.Register()` with constructor functions for both its Parser and Formatter. New plugins must:

1. Create `pkg/plugins/<name>/` with `<name>parser.go` and `<name>formatter.go`.
2. Define `newParser(cfg Config) (Parser, error)` and `newFormatter(cfg Config) (Formatter, error)`.
3. Call `plugins.Register(pluginName, newParser, newFormatter)` in `init()`.
4. Import the package from `pkg/core/core.go` (blank import, side-effect registration).

### Metric System

Prometheus metrics are modeled as Go structs embedding `exporter.MetricCommons`. Fields tagged with `label:"<key>"` are extracted via reflection as Prometheus labels at collection time. `MetricCommons` carries Name, Help, Device, Type (Counter/Gauge), and Value. The exporter builds the FQName as `<prefix>_<name>_total` (counters) or `<prefix>_<name>` (gauges).

### YANG Code Generation

`pkg/datamodels/ysocif/` and `pkg/datamodels/ysoclldp/` contain yGot-generated Go structs from OpenConfig YANG models. The `gen.go` files have `//go:generate` directives. **Do not hand-edit generated files.** Re-run `make gen_ysocif` / `make gen_ysoclldp` to regenerate after YANG model changes.

### Configuration

YAML config has three levels: global → `device_template` (shared defaults) → per-device overrides. Key relationships:
- `scrape_interval` must match the Prometheus scrape interval; gNMI `sample_interval` = `scrape_interval / oversampling`.
- `mode: cache` requires the gNMI stream to send DELETE messages when data disappears; `mode: passthrough` (default) re-parses buffered notifications on every scrape.
- `max_life` triggers periodic cache flush and reconnection to prevent stale data accumulation.
- See `config-keys.yaml` for all config options and `plugin-options.yaml` for per-plugin options.

### Self-Monitoring

Both `GnmiClient` and each `Plugin` emit their own Prometheus metrics (connection errors, parse failures, series counts, etc.) via the same `GMetricSource` interface. These are always registered regardless of plugin configuration.

<!-- code-review-graph MCP tools -->
## MCP Tools: code-review-graph

**IMPORTANT: This project has a knowledge graph. ALWAYS use the
code-review-graph MCP tools BEFORE using Grep/Glob/Read to explore
the codebase.** The graph is faster, cheaper (fewer tokens), and gives
you structural context (callers, dependents, test coverage) that file
scanning cannot.

### When to use graph tools FIRST

- **Exploring code**: `semantic_search_nodes` or `query_graph` instead of Grep
- **Understanding impact**: `get_impact_radius` instead of manually tracing imports
- **Code review**: `detect_changes` + `get_review_context` instead of reading entire files
- **Finding relationships**: `query_graph` with callers_of/callees_of/imports_of/tests_for
- **Architecture questions**: `get_architecture_overview` + `list_communities`

Fall back to Grep/Glob/Read **only** when the graph doesn't cover what you need.

### Key Tools

| Tool                        | Use when                                               |
|-----------------------------|--------------------------------------------------------|
| `detect_changes`            | Reviewing code changes — gives risk-scored analysis    |
| `get_review_context`        | Need source snippets for review — token-efficient      |
| `get_impact_radius`         | Understanding blast radius of a change                 |
| `get_affected_flows`        | Finding which execution paths are impacted             |
| `query_graph`               | Tracing callers, callees, imports, tests, dependencies |
| `semantic_search_nodes`     | Finding functions/classes by name or keyword           |
| `get_architecture_overview` | Understanding high-level codebase structure            |
| `refactor_tool`             | Planning renames, finding dead code                    |

### Workflow

1. The graph auto-updates on file changes (via hooks).
2. Use `detect_changes` for code review.
3. Use `get_affected_flows` to understand impact.
4. Use `query_graph` pattern="tests_for" to check coverage.
