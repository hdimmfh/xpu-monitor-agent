package plugin

import "context"

type Plugin interface {
	Name() string
	Discover(ctx context.Context) ([]Device, error)
	Capabilities(ctx context.Context, deviceID string) ([]Capability, error)
	Collect(ctx context.Context, deviceID string) ([]Metric, error)
}
