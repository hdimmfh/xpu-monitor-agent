package prometheus

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	coreplugin "github.com/hdimmfh/xpu-monitor-agent/pkg/plugin"
	prom "github.com/prometheus/client_golang/prometheus"
)

const (
	defaultCollectTimeout = 10 * time.Second
)

// MetricCollector is the minimum interface required by the Prometheus
// exporter.
//
// pkg/collector.Collector satisfies this interface through CollectAll.
type MetricCollector interface {
	CollectAll(ctx context.Context) ([]coreplugin.Metric, error)
}

// Collector converts XPUMON metrics into Prometheus metrics.
//
// A new XPUMON collection is performed for every Prometheus scrape.
type Collector struct {
	collector MetricCollector
	timeout   time.Duration

	upDesc             *prom.Desc
	scrapeDurationDesc *prom.Desc
	scrapeMetricsDesc  *prom.Desc
	scrapeErrorsDesc   *prom.Desc
}

// New creates a Prometheus collector backed by an XPUMON metric collector.
func New(
	metricCollector MetricCollector,
	timeout time.Duration,
) *Collector {
	if timeout <= 0 {
		timeout = defaultCollectTimeout
	}

	return &Collector{
		collector: metricCollector,
		timeout:   timeout,

		upDesc: prom.NewDesc(
			"xpumon_up",
			"Whether the most recent XPUMON collection succeeded.",
			nil,
			nil,
		),

		scrapeDurationDesc: prom.NewDesc(
			"xpumon_scrape_duration_seconds",
			"Duration of the most recent XPUMON metric collection.",
			nil,
			nil,
		),

		scrapeMetricsDesc: prom.NewDesc(
			"xpumon_scrape_metrics",
			"Number of device metrics returned by the most recent XPUMON collection.",
			nil,
			nil,
		),

		scrapeErrorsDesc: prom.NewDesc(
			"xpumon_scrape_errors",
			"Number of errors encountered during the most recent XPUMON collection.",
			nil,
			nil,
		),
	}
}

// Describe sends the fixed exporter descriptors.
//
// Device metric descriptors are generated dynamically because XPUMON plugins
// can introduce additional vendor-specific metrics.
func (c *Collector) Describe(ch chan<- *prom.Desc) {
	ch <- c.upDesc
	ch <- c.scrapeDurationDesc
	ch <- c.scrapeMetricsDesc
	ch <- c.scrapeErrorsDesc
}

// Collect collects XPUMON metrics and converts them into Prometheus samples.
func (c *Collector) Collect(ch chan<- prom.Metric) {
	startedAt := time.Now()

	if c.collector == nil {
		ch <- prom.MustNewConstMetric(
			c.upDesc,
			prom.GaugeValue,
			0,
		)
		ch <- prom.MustNewConstMetric(
			c.scrapeDurationDesc,
			prom.GaugeValue,
			time.Since(startedAt).Seconds(),
		)
		ch <- prom.MustNewConstMetric(
			c.scrapeMetricsDesc,
			prom.GaugeValue,
			0,
		)
		ch <- prom.MustNewConstMetric(
			c.scrapeErrorsDesc,
			prom.GaugeValue,
			1,
		)

		log.Printf("Prometheus collection failed: XPUMON collector is nil")
		return
	}

	ctx, cancel := context.WithTimeout(
		context.Background(),
		c.timeout,
	)
	defer cancel()

	metrics, err := c.collector.CollectAll(ctx)

	up := 1.0
	errorCount := 0.0

	if err != nil {
		up = 0
		errorCount = 1

		log.Printf(
			"Prometheus collection failed: %v",
			err,
		)
	}

	exportedCount := 0

	for _, metric := range metrics {
		promMetric, convertErr := convertMetric(metric)
		if convertErr != nil {
			errorCount++

			log.Printf(
				"skip unsupported metric device=%q name=%q unit=%q: %v",
				metric.DeviceID,
				metric.Name,
				metric.Unit,
				convertErr,
			)

			continue
		}

		ch <- promMetric
		exportedCount++
	}

	ch <- prom.MustNewConstMetric(
		c.upDesc,
		prom.GaugeValue,
		up,
	)

	ch <- prom.MustNewConstMetric(
		c.scrapeDurationDesc,
		prom.GaugeValue,
		time.Since(startedAt).Seconds(),
	)

	ch <- prom.MustNewConstMetric(
		c.scrapeMetricsDesc,
		prom.GaugeValue,
		float64(exportedCount),
	)

	ch <- prom.MustNewConstMetric(
		c.scrapeErrorsDesc,
		prom.GaugeValue,
		errorCount,
	)
}

func convertMetric(
	metric coreplugin.Metric,
) (prom.Metric, error) {
	name, help, value, err := normalizeMetric(metric)
	if err != nil {
		return nil, err
	}

	desc := prom.NewDesc(
		name,
		help,
		[]string{"device_id"},
		nil,
	)

	promMetric, err := prom.NewConstMetric(
		desc,
		prom.GaugeValue,
		value,
		metric.DeviceID,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"create Prometheus metric: %w",
			err,
		)
	}

	return promMetric, nil
}

func normalizeMetric(
	metric coreplugin.Metric,
) (
	name string,
	help string,
	value float64,
	err error,
) {
	metricName := normalizeIdentifier(metric.Name)
	unit := strings.ToLower(
		strings.TrimSpace(metric.Unit),
	)

	if metricName == "" {
		return "", "", 0, fmt.Errorf(
			"metric name is empty",
		)
	}

	switch {
	case isUtilizationMetric(metricName, unit):
		return normalizeUtilization(metricName, metric.Value, unit)

	case isByteMetric(metricName, unit):
		return normalizeByteMetric(metricName, metric.Value)

	case isTemperatureMetric(metricName, unit):
		return normalizeTemperatureMetric(metricName, metric.Value)

	case isPowerMetric(metricName, unit):
		return normalizePowerMetric(metricName, metric.Value)

	case isClockMetric(metricName, unit):
		return normalizeClockMetric(metricName, metric.Value, unit)

	default:
		prometheusName := "xpumon_device_" + metricName

		return prometheusName,
			fmt.Sprintf(
				"XPUMON device metric %s.",
				metric.Name,
			),
			metric.Value,
			nil
	}
}

func normalizeUtilization(
	metricName string,
	value float64,
	unit string,
) (
	string,
	string,
	float64,
	error,
) {
	normalizedName := stripSuffixes(
		metricName,
		"_percent",
		"_percentage",
		"_ratio",
	)

	switch unit {
	case "percent", "percentage", "%":
		value /= 100

	case "ratio", "":
		// Already represented as a ratio, or the plugin did not specify a unit.

	default:
		return "", "", 0, fmt.Errorf(
			"unsupported utilization unit %q",
			unit,
		)
	}

	return "xpumon_device_" + normalizedName + "_ratio",
		fmt.Sprintf(
			"XPUMON device %s as a ratio from 0 to 1.",
			strings.ReplaceAll(normalizedName, "_", " "),
		),
		value,
		nil
}

func normalizeByteMetric(
	metricName string,
	value float64,
) (
	string,
	string,
	float64,
	error,
) {
	normalizedName := stripSuffixes(
		metricName,
		"_byte",
		"_bytes",
	)

	return "xpumon_device_" + normalizedName + "_bytes",
		fmt.Sprintf(
			"XPUMON device %s in bytes.",
			strings.ReplaceAll(normalizedName, "_", " "),
		),
		value,
		nil
}

func normalizeTemperatureMetric(
	metricName string,
	value float64,
) (
	string,
	string,
	float64,
	error,
) {
	normalizedName := stripSuffixes(
		metricName,
		"_celsius",
		"_degree_celsius",
		"_degrees_celsius",
	)

	return "xpumon_device_" + normalizedName + "_celsius",
		fmt.Sprintf(
			"XPUMON device %s in degrees Celsius.",
			strings.ReplaceAll(normalizedName, "_", " "),
		),
		value,
		nil
}

func normalizePowerMetric(
	metricName string,
	value float64,
) (
	string,
	string,
	float64,
	error,
) {
	normalizedName := stripSuffixes(
		metricName,
		"_watt",
		"_watts",
	)

	return "xpumon_device_" + normalizedName + "_watts",
		fmt.Sprintf(
			"XPUMON device %s in watts.",
			strings.ReplaceAll(normalizedName, "_", " "),
		),
		value,
		nil
}

func normalizeClockMetric(
	metricName string,
	value float64,
	unit string,
) (
	string,
	string,
	float64,
	error,
) {
	normalizedName := stripSuffixes(
		metricName,
		"_hz",
		"_khz",
		"_mhz",
		"_ghz",
	)

	switch unit {
	case "hz", "hertz", "":
		// Already in hertz.

	case "khz", "kilohertz":
		value *= 1_000

	case "mhz", "megahertz":
		value *= 1_000_000

	case "ghz", "gigahertz":
		value *= 1_000_000_000

	default:
		return "", "", 0, fmt.Errorf(
			"unsupported clock unit %q",
			unit,
		)
	}

	return "xpumon_device_" + normalizedName + "_hertz",
		fmt.Sprintf(
			"XPUMON device %s in hertz.",
			strings.ReplaceAll(normalizedName, "_", " "),
		),
		value,
		nil
}

func isUtilizationMetric(
	name string,
	unit string,
) bool {
	if unit == "percent" ||
		unit == "percentage" ||
		unit == "%" ||
		unit == "ratio" {
		return true
	}

	return strings.Contains(name, "utilization")
}

func isByteMetric(
	name string,
	unit string,
) bool {
	if unit == "byte" || unit == "bytes" {
		return true
	}

	return strings.Contains(name, "memory") &&
		!strings.Contains(name, "utilization")
}

func isTemperatureMetric(
	name string,
	unit string,
) bool {
	switch unit {
	case "celsius",
		"degree_celsius",
		"degrees_celsius",
		"°c":
		return true
	}

	return strings.Contains(name, "temperature")
}

func isPowerMetric(
	name string,
	unit string,
) bool {
	switch unit {
	case "watt", "watts", "w":
		return true
	}

	return strings.Contains(name, "power")
}

func isClockMetric(
	name string,
	unit string,
) bool {
	switch unit {
	case "hz",
		"hertz",
		"khz",
		"kilohertz",
		"mhz",
		"megahertz",
		"ghz",
		"gigahertz":
		return true
	}

	return strings.Contains(name, "clock")
}

func normalizeIdentifier(value string) string {
	value = strings.TrimSpace(value)
	value = strings.ToLower(value)

	var builder strings.Builder
	builder.Grow(len(value))

	lastUnderscore := false

	for _, char := range value {
		isAlphaNumeric :=
			char >= 'a' && char <= 'z' ||
				char >= '0' && char <= '9'

		if isAlphaNumeric {
			builder.WriteRune(char)
			lastUnderscore = false
			continue
		}

		if !lastUnderscore {
			builder.WriteByte('_')
			lastUnderscore = true
		}
	}

	return strings.Trim(
		builder.String(),
		"_",
	)
}

func stripSuffixes(
	value string,
	suffixes ...string,
) string {
	for _, suffix := range suffixes {
		value = strings.TrimSuffix(
			value,
			suffix,
		)
	}

	return value
}
