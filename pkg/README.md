# XPUMON Core Package

The `pkg` directory contains the core abstractions shared across XPUMON.

Its purpose is to provide vendor-neutral interfaces and common data models that allow hardware-specific plugins to integrate with the monitoring framework without modifying the core agent.

## Responsibilities

- Define the common plugin interface
- Define normalized device and metric models
- Provide shared types used by all vendor plugins
- Keep the core independent of vendor-specific SDKs

## Design Principles

- **Vendor-neutral** — no dependency on a specific hardware vendor.
- **Extensible** — new accelerator vendors can be added by implementing the plugin interface.
- **Minimal** — only common abstractions belong in this package.
- **Reusable** — shared across the agent and all plugins.

## Directory Structure

```
pkg/plugin
├── capability.go   # Capability definitions
├── device.go       # Device model
├── metric.go       # Metric model
└── plugin.go       # Plugin interface
```

Vendor-specific implementations (e.g. NVIDIA, AMD, Intel) should remain outside this package and implement the interfaces defined here.
