# Core Package

`pkg` contains the vendor-neutral core components of XPUMON.

It provides the common interfaces and shared data models used by all hardware-specific plugins.

## Directory Structure

```text
pkg/
├── plugin/      # Core plugin interfaces and shared models
├── mock/        # Reference plugin implementation
├── collector/   # Metric aggregation layer
└── README.md
```

## Package Overview

| Package | Description |
|---------|-------------|
| `plugin` | Defines the vendor-neutral plugin interface and shared telemetry models. |
| `mock` | Reference implementation of `plugin.Plugin` for testing and development. |
| `collector` | Discovers devices and aggregates metrics from registered plugins. |

## Design Principles

- Vendor-neutral core interfaces
- Extensible plugin architecture
- Shared telemetry data model
- No vendor-specific SDK dependencies

## Documentation

Detailed design documents are available under the `docs/` directory.

| Document | Description |
|----------|-------------|
| `docs/00-overview.md` | Project overview and goals |
| `docs/01-architecture.md` | System architecture and component relationships |
| `docs/02-plugin-api.md` | Plugin interface and data model specification |

For contributors implementing new accelerator plugins, start with **`docs/01-plugin-api.md`**.
