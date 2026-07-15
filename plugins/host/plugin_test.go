package host

import (
	"context"
	"testing"

	"github.com/hdimmfh/xpu-monitor-agent/pkg/plugin"
)

func TestPluginImplementsInterface(t *testing.T) {
	var _ plugin.Plugin = New()
}

func TestPluginName(t *testing.T) {
	p := New()

	if got := p.Name(); got != "host" {
		t.Fatalf("Name() = %q, want %q", got, "host")
	}
}

func TestDiscover(t *testing.T) {
	p := New()

	devices, err := p.Discover(context.Background())
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}

	if len(devices) != 1 {
		t.Fatalf("len(devices) = %d, want 1", len(devices))
	}

	if devices[0].ID != hostDeviceID {
		t.Errorf(
			"device ID = %q, want %q",
			devices[0].ID,
			hostDeviceID,
		)
	}

	if devices[0].Type != plugin.DeviceTypeHost {
		t.Errorf(
			"device type = %q, want %q",
			devices[0].Type,
			plugin.DeviceTypeHost,
		)
	}
}

func TestCapabilities(t *testing.T) {
	p := New()

	capabilities, err := p.Capabilities(
		context.Background(),
		hostDeviceID,
	)
	if err != nil {
		t.Fatalf("Capabilities() error = %v", err)
	}

	if len(capabilities) != 1 {
		t.Fatalf(
			"len(capabilities) = %d, want 1",
			len(capabilities),
		)
	}

	if capabilities[0].Name != "memory" {
		t.Errorf(
			"capability = %q, want %q",
			capabilities[0].Name,
			"memory",
		)
	}
}

func TestCollect(t *testing.T) {
	p := New()

	metrics, err := p.Collect(
		context.Background(),
		hostDeviceID,
	)
	if err != nil {
		t.Fatalf("Collect() error = %v", err)
	}

	if len(metrics) != 4 {
		t.Fatalf(
			"len(metrics) = %d, want 4",
			len(metrics),
		)
	}

	for _, metric := range metrics {
		if metric.DeviceID != hostDeviceID {
			t.Errorf(
				"metric DeviceID = %q, want %q",
				metric.DeviceID,
				hostDeviceID,
			)
		}

		if metric.Timestamp.IsZero() {
			t.Errorf(
				"metric %q has zero timestamp",
				metric.Name,
			)
		}
	}
}
