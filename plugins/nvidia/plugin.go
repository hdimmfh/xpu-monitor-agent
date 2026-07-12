package nvidia

import (
	"context"
	"fmt"

	"github.com/NVIDIA/go-nvml/pkg/nvml"

	"github.com/hdimmfh/xpu-monitor-agent/pkg/plugin"
)

const (
	pluginName = "nvidia"
	vendorName = "nvidia"
)

type Plugin struct {
	initialized bool
}

// New initializes NVML and creates an NVIDIA plugin.
func New() (*Plugin, error) {
	ret := nvml.Init()
	if ret != nvml.SUCCESS {
		return nil, fmt.Errorf(
			"initialize NVML: %s",
			nvml.ErrorString(ret),
		)
	}

	return &Plugin{
		initialized: true,
	}, nil
}

// Close releases resources initialized by NVML.
func (p *Plugin) Close() error {
	if !p.initialized {
		return nil
	}

	ret := nvml.Shutdown()
	if ret != nvml.SUCCESS {
		return fmt.Errorf(
			"shutdown NVML: %s",
			nvml.ErrorString(ret),
		)
	}

	p.initialized = false

	return nil
}

// Name returns the plugin name.
func (p *Plugin) Name() string {
	return pluginName
}

// Discover discovers NVIDIA GPU devices available through NVML.
func (p *Plugin) Discover(
	ctx context.Context,
) ([]plugin.Device, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	if !p.initialized {
		return nil, fmt.Errorf("NVML is not initialized")
	}

	count, ret := nvml.DeviceGetCount()
	if ret != nvml.SUCCESS {
		return nil, fmt.Errorf(
			"get NVIDIA device count: %s",
			nvml.ErrorString(ret),
		)
	}

	devices := make([]plugin.Device, 0, count)

	for index := 0; index < count; index++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		device, ret := nvml.DeviceGetHandleByIndex(index)
		if ret != nvml.SUCCESS {
			return nil, fmt.Errorf(
				"get NVIDIA device handle at index %d: %s",
				index,
				nvml.ErrorString(ret),
			)
		}

		uuid, ret := device.GetUUID()
		if ret != nvml.SUCCESS {
			return nil, fmt.Errorf(
				"get NVIDIA device UUID at index %d: %s",
				index,
				nvml.ErrorString(ret),
			)
		}

		name, ret := device.GetName()
		if ret != nvml.SUCCESS {
			return nil, fmt.Errorf(
				"get NVIDIA device name at index %d: %s",
				index,
				nvml.ErrorString(ret),
			)
		}

		devices = append(
			devices,
			plugin.Device{
				ID:     uuid,
				Vendor: vendorName,
				Model:  name,
				Type:   plugin.DeviceTypeGPU,
			},
		)
	}

	return devices, nil
}

// Capabilities returns telemetry capabilities supported by the plugin.
func (p *Plugin) Capabilities(
	ctx context.Context,
	deviceID string,
) ([]plugin.Capability, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	if _, err := p.deviceByUUID(deviceID); err != nil {
		return nil, err
	}

	return []plugin.Capability{
		{Name: "utilization"},
		{Name: "memory"},
		{Name: "temperature"},
		{Name: "power"},
	}, nil
}

// deviceByUUID returns an NVML device handle using a stable GPU UUID.
func (p *Plugin) deviceByUUID(
	deviceID string,
) (nvml.Device, error) {
	if !p.initialized {
		return nil, fmt.Errorf(
			"NVML is not initialized",
		)
	}

	device, ret := nvml.DeviceGetHandleByUUID(deviceID)
	if ret != nvml.SUCCESS {
		return nil, fmt.Errorf(
			"get NVIDIA device %q: %s",
			deviceID,
			nvml.ErrorString(ret),
		)
	}

	return device, nil
}

var _ plugin.Plugin = (*Plugin)(nil)
