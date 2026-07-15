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
		)

		return nil

	case nvml.ERROR_NOT_SUPPORTED:
		return collectUnifiedSystemMemory(
			ctx,
			deviceID,
			timestamp,
			metrics,
		)

	default:
		return fmt.Errorf(
			"get memory info for NVIDIA device %q: %s",
			deviceID,
			nvml.ErrorString(ret),
		)
	}
}

func collectUnifiedSystemMemory(
	ctx context.Context,
	deviceID string,
	timestamp time.Time,
	metrics *[]plugin.Metric,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	memory, err := readSystemMemoryInfo()
	if err != nil {
		return fmt.Errorf(
			"read unified system memory for NVIDIA device %q: %w",
			deviceID,
			err,
		)
	}

	*metrics = append(
		*metrics,
		plugin.Metric{
			DeviceID:  deviceID,
			Name:      "unified_memory_used",
			Value:     memory.UsedBytes,
			Unit:      "byte",
			Timestamp: timestamp,
		},
		plugin.Metric{
			DeviceID:  deviceID,
			Name:      "unified_memory_total",
			Value:     memory.TotalBytes,
			Unit:      "byte",
			Timestamp: timestamp,
		},
		plugin.Metric{
			DeviceID:  deviceID,
			Name:      "unified_memory_available",
			Value:     memory.AvailableBytes,
			Unit:      "byte",
			Timestamp: timestamp,
		},
	)

	return nil
}
