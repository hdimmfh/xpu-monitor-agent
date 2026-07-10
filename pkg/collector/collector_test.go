package collector

import (
	"context"
	"testing"

	"github.com/hdimmfh/xpu-monitor-agent/pkg/mock"
)

func TestDiscoverAll(t *testing.T) {
	c := New(mock.New())

	devices, err := c.DiscoverAll(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(devices) == 0 {
		t.Fatal("expected discovered devices")
	}
}

func TestCollectAll(t *testing.T) {
	c := New(mock.New())

	metrics, err := c.CollectAll(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(metrics) == 0 {
		t.Fatal("expected collected metrics")
	}
}
