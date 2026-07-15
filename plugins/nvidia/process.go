package nvidia

import (
	"context"
	"fmt"
	"sort"
	"strconv"

	"github.com/NVIDIA/go-nvml/pkg/nvml"
	"github.com/hdimmfh/xpu-monitor-agent/pkg/plugin"
)

// Processes returns processes currently using the specified NVIDIA device.
//
// Compute and graphics process lists are both queried. A process returned by
// both NVML APIs is deduplicated by PID.
func (p *Plugin) Processes(
	ctx context.Context,
	deviceID string,
) ([]plugin.DeviceProcess, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	device, err := p.deviceByUUID(deviceID)
	if err != nil {
		return nil, err
	}

	processesByPID := make(
		map[int]plugin.DeviceProcess,
	)

	computeProcesses, ret := device.GetComputeRunningProcesses()
	switch ret {
	case nvml.SUCCESS:
		mergeProcessInfo(
			processesByPID,
			deviceID,
			computeProcesses,
		)

	case nvml.ERROR_NOT_SUPPORTED:
		// Continue because graphics process discovery may still be
		// available on this device.

	default:
		return nil, fmt.Errorf(
			"get compute processes for NVIDIA device %q: %s",
			deviceID,
			nvml.ErrorString(ret),
		)
	}

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	graphicsProcesses, ret := device.GetGraphicsRunningProcesses()
	switch ret {
	case nvml.SUCCESS:
		mergeProcessInfo(
			processesByPID,
			deviceID,
			graphicsProcesses,
		)

	case nvml.ERROR_NOT_SUPPORTED:
		// Headless accelerator devices may not expose graphics process
		// discovery.

	default:
		return nil, fmt.Errorf(
			"get graphics processes for NVIDIA device %q: %s",
			deviceID,
			nvml.ErrorString(ret),
		)
	}

	processes := make(
		[]plugin.DeviceProcess,
		0,
		len(processesByPID),
	)

	for _, detectedProcess := range processesByPID {
		processes = append(
			processes,
			detectedProcess,
		)
	}

	sort.Slice(
		processes,
		func(i, j int) bool {
			return processes[i].PID < processes[j].PID
		},
	)

	return processes, nil
}

func mergeProcessInfo(
	processesByPID map[int]plugin.DeviceProcess,
	deviceID string,
	processInfos []nvml.ProcessInfo,
) {
	for _, processInfo := range processInfos {
		pid := int(processInfo.Pid)
		if pid <= 0 {
			continue
		}

		candidate := plugin.DeviceProcess{
			PID:             pid,
			DeviceID:        deviceID,
			UsedMemoryBytes: normalizeUsedMemory(processInfo.UsedGpuMemory),
			Metadata:        make(map[string]string),
		}

		if gpuInstanceID, available := normalizeInstanceID(
			processInfo.GpuInstanceId,
		); available {
			candidate.Metadata["gpu_instance_id"] = strconv.FormatUint(
				uint64(gpuInstanceID),
				10,
			)
		}

		if computeInstanceID, available := normalizeInstanceID(
			processInfo.ComputeInstanceId,
		); available {
			candidate.Metadata["compute_instance_id"] = strconv.FormatUint(
				uint64(computeInstanceID),
				10,
			)
		}

		if len(candidate.Metadata) == 0 {
			candidate.Metadata = nil
		}

		existing, found := processesByPID[pid]
		if !found {
			processesByPID[pid] = candidate
			continue
		}

		// The same PID can be returned by both compute and graphics queries.
		// Do not sum memory because the two entries can represent the same
		// device allocation.
		if candidate.UsedMemoryBytes > existing.UsedMemoryBytes {
			existing.UsedMemoryBytes = candidate.UsedMemoryBytes
		}

		if len(candidate.Metadata) > 0 {
			if existing.Metadata == nil {
				existing.Metadata = make(map[string]string)
			}

			for key, value := range candidate.Metadata {
				existing.Metadata[key] = value
			}
		}

		processesByPID[pid] = existing
	}
}

func normalizeUsedMemory(
	value uint64,
) uint64 {
	// NVML uses UINT64_MAX when process memory usage is unavailable.
	if value == ^uint64(0) {
		return 0
	}

	return value
}

func normalizeInstanceID(
	value uint32,
) (uint32, bool) {
	// NVML uses UINT32_MAX when the process is not associated with
	// a MIG GPU instance or compute instance.
	if value == ^uint32(0) {
		return 0, false
	}

	return value, true
}

var _ plugin.ProcessProvider = (*Plugin)(nil)
