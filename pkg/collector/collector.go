package collector

import (
	"context"
	"errors"
	"fmt"

	"github.com/hdimmfh/xpu-monitor-agent/pkg/plugin"
)

type Collector struct {
	plugins []plugin.Plugin
}

func New(
	plugins ...plugin.Plugin,
) *Collector {
	return &Collector{
		plugins: plugins,
	}
}

func (c *Collector) DiscoverAll(
	ctx context.Context,
) ([]plugin.Device, error) {
	var devices []plugin.Device
	var discoveryErrors []error

	for _, p := range c.plugins {
		discovered, err := p.Discover(ctx)
		if err != nil {
			discoveryErrors = append(
				discoveryErrors,
				fmt.Errorf(
					"discover devices with plugin %q: %w",
					p.Name(),
					err,
				),
			)

			continue
		}

		devices = append(
			devices,
			discovered...,
		)
	}

	return devices, errors.Join(
		discoveryErrors...,
	)
}

func (c *Collector) CollectAll(
	ctx context.Context,
) ([]plugin.Metric, error) {
	var metrics []plugin.Metric
	var collectionErrors []error

	for _, p := range c.plugins {
		devices, err := p.Discover(ctx)
		if err != nil {
			collectionErrors = append(
				collectionErrors,
				fmt.Errorf(
					"discover devices with plugin %q: %w",
					p.Name(),
					err,
				),
			)

			continue
		}

		for _, device := range devices {
			collected, err := p.Collect(
				ctx,
				device.ID,
			)
			if err != nil {
				collectionErrors = append(
					collectionErrors,
					fmt.Errorf(
						"collect device %q with plugin %q: %w",
						device.ID,
						p.Name(),
						err,
					),
				)

				continue
			}

			metrics = append(
				metrics,
				collected...,
			)
		}
	}

	return metrics, errors.Join(
		collectionErrors...,
	)
}
