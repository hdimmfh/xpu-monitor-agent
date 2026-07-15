package host

import (
	"context"
	"fmt"
	"time"

	"github.com/hdimmfh/xpu-monitor-agent/pkg/plugin"
)

func (p *Plugin) Collect(
	ctx context.Context,
	deviceID string,
) ([]plugin.Metric, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	if deviceID != hostDeviceID {
		return nil, fmt.Errorf(
			"unknown host device %q",
			deviceID,
		)
	}

	memory, err := readSystemMemoryInfo()
	if err != nil {
		return nil, fmt.Errorf(
			"collect host memory: %w",
			err,
		)
	}

	now := time.Now()

	return []plugin.Metric{
		{
			DeviceID:  deviceID,
			Name:      "memory_total",
			Value:     memory.TotalBytes,
			Unit:      "byte",
			Timestamp: now,
		},
		{
			DeviceID:  deviceID,
			Name:      "memory_used",
			Value:     memory.UsedBytes,
			Unit:      "byte",
			Timestamp: now,
		},
		{
			DeviceID:  deviceID,
			Name:      "memory_free",
			Value:     memory.FreeBytes,
			Unit:      "byte",
			Timestamp: now,
		},
		{
			DeviceID:  deviceID,
			Name:      "memory_available",
			Value:     memory.AvailableBytes,
			Unit:      "byte",
			Timestamp: now,
		},
	}, nil
}
