package nvidia

import (
	"context"
	"fmt"
	"time"

	"github.com/NVIDIA/go-nvml/pkg/nvml"

	"github.com/hdimmfh/xpu-monitor-agent/pkg/plugin"
)

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
