// plugins/mock/mock_test.go
package mock

import (
	"context"
	"testing"

	"github.com/hdimmfh/xpu-monitor-agent/pkg/plugin"
)

func TestPluginImplementsInterface(t *testing.T) {
	var _ plugin.Plugin = New()
}

func TestName(t *testing.T) {
	p := New()

	if got := p.Name(); got != "mock" {
		t.Fatalf("expected plugin name mock, got %s", got)
	}
}

func TestDiscover(t *testing.T) {
	p := New()

	devices, err := p.Discover(context.Background())
	if err != nil {
		t.Fatalf("discover failed: %v", err)
	}

	if len(devices) != 1 {
		t.Fatalf("expected 1 device, got %d", len(devices))
	}

	if devices[0].ID != "mock-0" {
		t.Fatalf("expected device ID mock-0, got %s", devices[0].ID)
	}

	if devices[0].Vendor != "mock" {
		t.Fatalf("expected vendor mock, got %s", devices[0].Vendor)
	}
}

func TestCapabilities(t *testing.T) {
	p := New()

	caps, err := p.Capabilities(context.Background(), "mock-0")
	if err != nil {
		t.Fatalf("capabilities failed: %v", err)
	}

	if len(caps) == 0 {
		t.Fatal("expected at least one capability")
	}
}

func TestCapabilitiesUnknownDevice(t *testing.T) {
	p := New()

	_, err := p.Capabilities(context.Background(), "unknown")
	if err == nil {
		t.Fatal("expected error for unknown device")
	}
}

func TestCollect(t *testing.T) {
	p := New()

	metrics, err := p.Collect(context.Background(), "mock-0")
	if err != nil {
		t.Fatalf("collect failed: %v", err)
	}

	if len(metrics) == 0 {
		t.Fatal("expected at least one metric")
	}

	for _, m := range metrics {
		if m.Name == "" {
			t.Fatal("metric name must not be empty")
		}

		if m.Unit == "" {
			t.Fatalf("metric %s unit must not be empty", m.Name)
		}

		if m.Timestamp.IsZero() {
			t.Fatalf("metric %s timestamp must not be zero", m.Name)
		}
	}
}

func TestCollectUnknownDevice(t *testing.T) {
	p := New()

	_, err := p.Collect(context.Background(), "unknown")
	if err == nil {
		t.Fatal("expected error for unknown device")
	}
}

func TestContextCancellation(t *testing.T) {
	p := New()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := p.Discover(ctx); err == nil {
		t.Fatal("expected context cancellation error")
	}
}
