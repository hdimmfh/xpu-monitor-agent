type Plugin interface {
    Name() string
    Discover(ctx context.Context) ([]Device, error)
    Collect(ctx context.Context, deviceID string) ([]Metric, error)
}
