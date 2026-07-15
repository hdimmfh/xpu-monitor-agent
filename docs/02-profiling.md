# Python Profiling

## Overview

XPUMON integrates with `py-spy` to profile running Python workloads.

Profiling complements hardware telemetry by providing visibility into the Python execution stack associated with an active workload.

Currently supported profiling modes:

- `dump` — captures an instantaneous Python stack snapshot.
- `record` — samples a Python process over a configurable duration.

---

## Process Discovery

Before profiling, XPUMON discovers candidate Python processes.

Process discovery is configurable through the `discovery` section.

Supported filters include:

- Process ID
- User
- Command regular expression
- Executable regular expression

Example configuration:

```yaml
profiling:
  discovery:
    enabled: true
    proc_root: /proc

    exclude:
      pids: []
      users: []
      command_regex:
        - "ipykernel"
        - "jupyter.*kernel"
      executable_regex: []
```

See:

- `configs/pyspy-dump.yaml`
- `configs/pyspy-record.yaml`

---

## Dump Mode

`dump` captures the current Python stack of a target process.

Typical use cases:

- Debugging
- Snapshot inspection
- Detecting synchronization points
- Repeated stack collection

Run:

```bash
./xpumon profile --config ./configs/pyspy-dump.yaml
```

Repeated snapshots can be collected using:

```bash
watch -n 1 './xpumon profile --config ./configs/pyspy-dump.yaml'
```

---

## Record Mode

`record` samples a Python process for a configured duration.

Typical use cases:

- Performance investigation
- Identifying frequently executed functions
- CPU-side hotspot analysis

Run:

```bash
./xpumon profile --config ./configs/pyspy-record.yaml
```

---

## Configuration

Profiling is configured through YAML.

Example configurations are available in:

- `configs/pyspy-dump.yaml`
- `configs/pyspy-record.yaml`

Configuration includes:

- Process discovery
- Process exclusion filters
- py-spy binary
- Profiling mode
- Sampling duration
- Sampling rate
- Native stack collection

---

## Profile Output

XPUMON wraps raw `py-spy` output with metadata.

Example:

```text
profile=py-spy pid=206450 format=text started_at=... ended_at=...
profile_data_begin
...
profile_data_end
```

The metadata makes it easier to correlate profile output with devices, processes, containers, and jobs.

---

## Limitations

`py-spy` profiles Python and native CPU-side stacks.

It does **not** profile GPU kernels directly.

GPU kernel execution should be analyzed with GPU profilers such as NVIDIA Nsight Systems or Nsight Compute.

When a stack repeatedly shows:

```text
torch.cuda.synchronize()
```

it indicates that the sampled Python thread was waiting at a synchronization point, not that the synchronization call itself performed the GPU computation.

---

## Future Work

Planned profiling improvements include:

- Process-to-GPU correlation
- Container and Kubernetes metadata
- Continuous profiling
- Automatic profile collection
- Structured profile exporters
- OpenTelemetry integration
