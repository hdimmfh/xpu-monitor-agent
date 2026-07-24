package prometheus

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	coreplugin "github.com/hdimmfh/xpu-monitor-agent/pkg/plugin"
	prom "github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

type fakeMetricCollector struct {
	metrics []coreplugin.Metric
	err     error
}

func (f *fakeMetricCollector) CollectAll(
	ctx context.Context,
) ([]coreplugin.Metric, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	return f.metrics, f.err
}

func TestCollectorMemoryMetric(t *testing.T) {
	metricCollector := &fakeMetricCollector{
		metrics: []coreplugin.Metric{
			{
				DeviceID: "GPU-0",
				Name:     "memory_used",
				Value:    float64(4 * 1024 * 1024 * 1024),
				Unit:     "byte",
				Timestamp: time.Date(
					2026,
					time.July,
					16,
					1,
					0,
					0,
					0,
					time.UTC,
				),
			},
		},
	}

	collector := New(
		metricCollector,
		time.Second,
	)

	registry := prom.NewRegistry()

	if err := registry.Register(collector); err != nil {
		t.Fatalf(
			"register collector: %v",
			err,
		)
	}

	expected := `
# HELP xpumon_device_memory_used_bytes XPUMON device memory used in bytes.
# TYPE xpumon_device_memory_used_bytes gauge
xpumon_device_memory_used_bytes{device_id="GPU-0"} 4.294967296e+09
# HELP xpumon_scrape_errors Number of errors encountered during the most recent XPUMON collection.
# TYPE xpumon_scrape_errors gauge
xpumon_scrape_errors 0
# HELP xpumon_scrape_metrics Number of device metrics returned by the most recent XPUMON collection.
# TYPE xpumon_scrape_metrics gauge
xpumon_scrape_metrics 1
# HELP xpumon_up Whether the most recent XPUMON collection succeeded.
# TYPE xpumon_up gauge
xpumon_up 1
`

	err := testutil.GatherAndCompare(
		registry,
		strings.NewReader(expected),
		"xpumon_device_memory_used_bytes",
		"xpumon_scrape_errors",
		"xpumon_scrape_metrics",
		"xpumon_up",
	)
	if err != nil {
		t.Fatalf(
			"unexpected gathered metrics: %v",
			err,
		)
	}
}

func TestCollectorUtilizationPercentToRatio(t *testing.T) {
	metricCollector := &fakeMetricCollector{
		metrics: []coreplugin.Metric{
			{
				DeviceID:  "GPU-0",
				Name:      "utilization",
				Value:     87,
				Unit:      "percent",
				Timestamp: time.Now().UTC(),
			},
		},
	}

	collector := New(
		metricCollector,
		time.Second,
	)

	registry := prom.NewRegistry()

	if err := registry.Register(collector); err != nil {
		t.Fatalf(
			"register collector: %v",
			err,
		)
	}

	expected := `
# HELP xpumon_device_utilization_ratio XPUMON device utilization as a ratio from 0 to 1.
# TYPE xpumon_device_utilization_ratio gauge
xpumon_device_utilization_ratio{device_id="GPU-0"} 0.87
# HELP xpumon_up Whether the most recent XPUMON collection succeeded.
# TYPE xpumon_up gauge
xpumon_up 1
`

	err := testutil.GatherAndCompare(
		registry,
		strings.NewReader(expected),
		"xpumon_device_utilization_ratio",
		"xpumon_up",
	)
	if err != nil {
		t.Fatalf(
			"unexpected gathered metrics: %v",
			err,
		)
	}
}

func TestCollectorCollectionFailure(t *testing.T) {
	metricCollector := &fakeMetricCollector{
		err: errors.New("collection failed"),
	}

	collector := New(
		metricCollector,
		time.Second,
	)

	registry := prom.NewRegistry()

	if err := registry.Register(collector); err != nil {
		t.Fatalf(
			"register collector: %v",
			err,
		)
	}

	expected := `
# HELP xpumon_scrape_errors Number of errors encountered during the most recent XPUMON collection.
# TYPE xpumon_scrape_errors gauge
xpumon_scrape_errors 1
# HELP xpumon_scrape_metrics Number of device metrics returned by the most recent XPUMON collection.
# TYPE xpumon_scrape_metrics gauge
xpumon_scrape_metrics 0
# HELP xpumon_up Whether the most recent XPUMON collection succeeded.
# TYPE xpumon_up gauge
xpumon_up 0
`

	err := testutil.GatherAndCompare(
		registry,
		strings.NewReader(expected),
		"xpumon_scrape_errors",
		"xpumon_scrape_metrics",
		"xpumon_up",
	)
	if err != nil {
		t.Fatalf(
			"unexpected gathered metrics: %v",
			err,
		)
	}
}

func TestNormalizeMetricMegahertz(t *testing.T) {
	metric := coreplugin.Metric{
		DeviceID: "GPU-0",
		Name:     "graphics_clock",
		Value:    1200,
		Unit:     "MHz",
	}

	name, _, value, err := normalizeMetric(metric)
	if err != nil {
		t.Fatalf(
			"normalize metric: %v",
			err,
		)
	}

	if name != "xpumon_device_graphics_clock_hertz" {
		t.Fatalf(
			"name = %q, want %q",
			name,
			"xpumon_device_graphics_clock_hertz",
		)
	}

	const expected = 1_200_000_000

	if value != expected {
		t.Fatalf(
			"value = %v, want %v",
			value,
			expected,
		)
	}
}

func TestNormalizeIdentifier(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "snake case",
			input: "memory_used",
			want:  "memory_used",
		},
		{
			name:  "spaces",
			input: "GPU Utilization",
			want:  "gpu_utilization",
		},
		{
			name:  "symbols",
			input: "power.draw",
			want:  "power_draw",
		},
		{
			name:  "repeated symbols",
			input: "GPU---Clock",
			want:  "gpu_clock",
		},
	}

	for _, test := range tests {
		t.Run(
			test.name,
			func(t *testing.T) {
				got := normalizeIdentifier(test.input)

				if got != test.want {
					t.Fatalf(
						"normalizeIdentifier(%q) = %q, want %q",
						test.input,
						got,
						test.want,
					)
				}
			},
		)
	}
}
