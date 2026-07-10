package collector

import (
	"context"

	"github.com/hdimmfh/xpu-monitor-agent/pkg/plugin"
)

type Collector struct {
	plugins []plugin.Plugin
}

func New(plugins ...plugin.Plugin) *Collector {
	return &Collector{
		plugins: plugins,
	}
}

func (c *Collector) DiscoverAll(ctx context.Context) ([]plugin.Device, error) {
	var devices []plugin.Device

	for _, p := range c.plugins {
		discovered, err := p.Discover(ctx)
		if err != nil {
			return nil, err
		}

		devices = append(devices, discovered...)
	}

	return devices, nil
}

func (c *Collector) CollectAll(ctx context.Context) ([]plugin.Metric, error) {
	var metrics []plugin.Metric

	for _, p := range c.plugins {
		devices, err := p.Discover(ctx)
		if err != nil {
			return nil, err
		}

		for _, device := range devices {
			collected, err := p.Collect(ctx, device.ID)
			if err != nil {
				return nil, err
			}

			metrics = append(metrics, collected...)
		}
	}

	return metrics, nil
}
