package nvidia

import (
	"context"
	"fmt"
	"time"

	"github.com/NVIDIA/go-nvml/pkg/nvml"
	"github.com/hdimmfh/xpu-monitor-agent/pkg/plugin"
)

func collectMemory(
	ctx context.Context,
	device nvml.Device,
	deviceID string,
	timestamp time.Time,
	metrics *[]plugin.Metric,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	memory, ret := device.GetMemoryInfo()

	switch ret {
	case nvml.SUCCESS:
		appendDeviceMemoryMetrics(
			metrics,
			deviceID,
			timestamp,
			memory.Total,
			memory.Used,
			memory.Free,
		)

		return nil

	case nvml.ERROR_NOT_SUPPORTED:
		// UMA devices may not expose a separate framebuffer-memory
		// pool through NVML.
		//
		// Fall back to Linux system memory because the CPU and GPU
		// share the same physical memory pool.
		systemMemory, err := readSystemMemoryInfo()
		if err != nil {
			return fmt.Errorf(
				"read unified system memory for NVIDIA device %q: %w",
				deviceID,
				err,
			)
		}

		appendUnifiedMemoryMetrics(
			metrics,
			deviceID,
			timestamp,
			systemMemory,
		)

		return nil

	default:
		return fmt.Errorf(
			"get memory info for NVIDIA device %q: %s",
			deviceID,
			nvml.ErrorString(ret),
		)
	}
}

func appendDeviceMemoryMetrics(
	metrics *[]plugin.Metric,
	deviceID string,
	timestamp time.Time,
	total uint64,
	used uint64,
	free uint64,
) {
	*metrics = append(
		*metrics,
		plugin.Metric{
			DeviceID:  deviceID,
			Name:      "memory_used",
			Value:     used,
			Unit:      "byte",
			Timestamp: timestamp,
		},
		plugin.Metric{
			DeviceID:  deviceID,
			Name:      "memory_total",
			Value:     total,
			Unit:      "byte",
			Timestamp: timestamp,
		},
		plugin.Metric{
			DeviceID:  deviceID,
			Name:      "memory_free",
			Value:     free,
			Unit:      "byte",
			Timestamp: timestamp,
		},
	)
}

func appendUnifiedMemoryMetrics(
	metrics *[]plugin.Metric,
	deviceID string,
	timestamp time.Time,
	memory systemMemoryInfo,
) {
	*metrics = append(
		*metrics,
		plugin.Metric{
			DeviceID:  deviceID,
			Name:      "memory_used",
			Value:     memory.UsedBytes,
			Unit:      "byte",
			Timestamp: timestamp,
		},
		plugin.Metric{
			DeviceID:  deviceID,
			Name:      "memory_total",
			Value:     memory.TotalBytes,
			Unit:      "byte",
			Timestamp: timestamp,
		},
		plugin.Metric{
			DeviceID:  deviceID,
			Name:      "memory_free",
			Value:     memory.FreeBytes,
			Unit:      "byte",
			Timestamp: timestamp,
		},
		plugin.Metric{
			DeviceID:  deviceID,
			Name:      "memory_available",
			Value:     memory.AvailableBytes,
			Unit:      "byte",
			Timestamp: timestamp,
		},
	)
}
