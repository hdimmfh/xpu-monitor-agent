package mock

import (
	"context"
	"fmt"
	"time"

	"github.com/hdimmfh/xpu-monitor-agent/pkg/plugin"
)

type Plugin struct {
	devices []plugin.Device
}

func New() *Plugin {
	return &Plugin{
		devices: []plugin.Device{
			{
				ID:     "mock-0",
				Vendor: "mock",
				Model:  "Mock Accelerator",
				Type:   plugin.DeviceTypeGPU,
			},
		},
	}
}

func (p *Plugin) Name() string {
	return "mock"
}

func (p *Plugin) Discover(ctx context.Context) ([]plugin.Device, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	return append([]plugin.Device(nil), p.devices...), nil
}

func (p *Plugin) Capabilities(ctx context.Context, deviceID string) ([]plugin.Capability, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	if !p.hasDevice(deviceID) {
		return nil, fmt.Errorf("device not found: %s", deviceID)
	}

	return []plugin.Capability{
		{Name: "temperature"},
		{Name: "power"},
		{Name: "memory"},
		{Name: "utilization"},
	}, nil
}

func (p *Plugin) Collect(ctx context.Context, deviceID string) ([]plugin.Metric, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	if !p.hasDevice(deviceID) {
		return nil, fmt.Errorf("device not found: %s", deviceID)
	}

	now := time.Now()

	return []plugin.Metric{
		{
			DeviceID:  deviceID,
			Name:      "temperature",
			Value:     65.0,
			Unit:      "celsius",
			Timestamp: now,
		},
		{
			DeviceID:  deviceID,
			Name:      "power",
			Value:     180.0,
			Unit:      "watt",
			Timestamp: now,
		},
		{
			DeviceID:  deviceID,
			Name:      "memory_used",
			Value:     uint64(4 * 1024 * 1024 * 1024),
			Unit:      "byte",
			Timestamp: now,
		},
		{
			DeviceID:  deviceID,
			Name:      "gpu_utilization",
			Value:     72.5,
			Unit:      "percent",
			Timestamp: now,
		},
	}, nil
}

func (p *Plugin) hasDevice(deviceID string) bool {
	for _, d := range p.devices {
		if d.ID == deviceID {
			return true
		}
	}

	return false
}

var _ plugin.Plugin = (*Plugin)(nil)
