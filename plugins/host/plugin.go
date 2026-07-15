package host

import (
	"context"
	"os"

	"github.com/hdimmfh/xpu-monitor-agent/pkg/plugin"
)

const (
	pluginName   = "host"
	hostDeviceID = "host"
	vendorName   = "generic"
)

type Plugin struct{}

var _ plugin.Plugin = (*Plugin)(nil)

func New() *Plugin {
	return &Plugin{}
}

func (p *Plugin) Name() string {
	return pluginName
}

func (p *Plugin) Discover(
	ctx context.Context,
) ([]plugin.Device, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	hostname, err := os.Hostname()
	if err != nil {
		return nil, err
	}

	return []plugin.Device{
		{
			ID:     hostDeviceID,
			Vendor: vendorName,
			Model:  hostname,
			Type:   plugin.DeviceTypeXPU,
		},
	}, nil
}

func (p *Plugin) Capabilities(
	ctx context.Context,
	deviceID string,
) ([]plugin.Capability, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	if deviceID != hostDeviceID {
		return nil, nil
	}

	return []plugin.Capability{
		{Name: "memory"},
	}, nil
}
