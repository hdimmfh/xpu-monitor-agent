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
		*metrics = append(
			*metrics,
			plugin.Metric{
				DeviceID:  deviceID,
				Name:      "memory_used",
				Value:     memory.Used,
				Unit:      "byte",
				Timestamp: timestamp,
			},
			plugin.Metric{
				DeviceID:  deviceID,
				Name:      "memory_total",
				Value:     memory.Total,
				Unit:      "byte",
				Timestamp: timestamp,
			},
			plugin.Metric{
				DeviceID:  deviceID,
				Name:      "memory_free",
				Value:     memory.Free,
				Unit:      "byte",
				Timestamp: timestamp,
			},
		)

		return nil

	case nvml.ERROR_NOT_SUPPORTED:
		// Integrated or unified-memory NVIDIA devices may not expose
		// device memory information through NVML.
		return nil

	default:
		return fmt.Errorf(
			"get memory info for NVIDIA device %q: %s",
			deviceID,
			nvml.ErrorString(ret),
		)
	}
}
