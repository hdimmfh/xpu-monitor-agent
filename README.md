# XPUMON

XPUMON is a vendor-neutral monitoring and profiling framework for heterogeneous AI infrastructure.

It provides a common plugin interface for discovering devices, collecting telemetry, and profiling Python workloads while isolating vendor-specific implementations behind plugins.

---

## Features

### Monitoring

- Vendor-neutral plugin architecture
- Host telemetry collection
- NVIDIA GPU telemetry through NVML
- Multi-device discovery
- Unified device, capability, and metric models

### Profiling

- Python process discovery
- Configurable process discovery and exclusion
- `py-spy dump` support
- `py-spy record` support
- YAML-based profiling configuration

---

## Architecture

```mermaid
flowchart LR
    Host["Host Plugin"]
    NVIDIA["NVIDIA Plugin"]
    Future["Future Plugins"]

    Host --> Core["XPUMON Core"]
    NVIDIA --> Core
    Future --> Core

    Core --> Metrics["Metrics"]
    Core --> Profiling["Profiling"]
```

Every telemetry source implements the same plugin interface.

```go
type Plugin interface {
    Name() string
    Discover(ctx context.Context) ([]Device, error)
    Capabilities(ctx context.Context, deviceID string) ([]Capability, error)
    Collect(ctx context.Context, deviceID string) ([]Metric, error)
}
```

---

## Repository Structure

```text
.
├── cmd/
├── configs/
├── docs/
├── pkg/
├── plugins/
│   ├── host/
│   └── nvidia/
└── README.md
```

---

## Quick Start

Build XPUMON:

```bash
go build -o xpumon ./cmd/xpumon
```

Collect metrics:

```bash
./xpumon
```

Run Python stack snapshot (`dump` mode):

```bash
./xpumon profile --config ./configs/pyspy-dump.yaml
```

Run sampling profiler (`record` mode):

```bash
./xpumon profile --config ./configs/pyspy-record.yaml
```

Run tests:

```bash
go test ./...
```

---

## Configuration

XPUMON uses YAML configuration for process discovery and profiling.

Example configurations are available in:

- [`configs/pyspy-dump.yaml`](configs/pyspy-dump.yaml)
- [`configs/pyspy-record.yaml`](configs/pyspy-record.yaml)

Configuration supports:

- Process discovery (`/proc`)
- Process exclusion by PID, user, command, or executable
- `py-spy` binary configuration
- `dump` and `record` profiling modes
- Native stack collection

---

## Roadmap

### Implemented

- Vendor-neutral plugin interface
- Host plugin
- NVIDIA NVML plugin
- Multi-device discovery
- Host and GPU telemetry collection
- Python process discovery
- Configurable process exclusion
- `py-spy dump` integration
- `py-spy record` integration
- YAML-based configuration

### Planned

- Prometheus exporter
- OpenTelemetry exporter
- Kubernetes integration
- Process-to-GPU correlation
- Additional accelerator plugins

---

## Documentation

- [Project Overview](docs/00-overview.md)
- [Plugin API](docs/01-plugin-api.md)

More documentation will be added as the project evolves.

---

## License

Licensed under the Apache License 2.0. See the [LICENSE](LICENSE) file for details.
