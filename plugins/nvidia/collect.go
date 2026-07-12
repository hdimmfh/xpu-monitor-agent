package nvidia

import (
	"context"
	"fmt"
	"time"

	"github.com/NVIDIA/go-nvml/pkg/nvml"

	"github.com/hdimmfh/xpu-monitor-agent/pkg/plugin"
)

// Collect collects normalized telemetry from one NVIDIA GPU.
func (p *Plugin) Collect(
	ctx context.Context,
	deviceID string,
) ([]plugin.Metric, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	device, err := p.deviceByUUID(deviceID)
	if err != nil {
		return nil, err
	}

	now := time.Now()

	metrics := make([]plugin.Metric, 0, 6)

	utilization, ret := device.GetUtilizationRates()
	if ret != nvml.SUCCESS {
		return nil, fmt.Errorf(
			"get utilization for NVIDIA device %q: %s",
			deviceID,
			nvml.ErrorString(ret),
		)
	}

	metrics = append(
		metrics,
		plugin.Metric{
			DeviceID:  deviceID,
			Name:      "gpu_utilization",
			Value:     float64(utilization.Gpu),
			Unit:      "percent",
			Timestamp: now,
		},
		plugin.Metric{
			DeviceID:  deviceID,
			Name:      "memory_utilization",
			Value:     float64(utilization.Memory),
			Unit:      "percent",
			Timestamp: now,
		},
	)

	memory, ret := device.GetMemoryInfo()
	if ret != nvml.SUCCESS {
		return nil, fmt.Errorf(
			"get memory info for NVIDIA device %q: %s",
			deviceID,
			nvml.ErrorString(ret),
		)
	}

	metrics = append(
		metrics,
		plugin.Metric{
			DeviceID:  deviceID,
			Name:      "memory_used",
			Value:     memory.Used,
			Unit:      "byte",
			Timestamp: now,
		},
		plugin.Metric{
			DeviceID:  deviceID,
			Name:      "memory_total",
			Value:     memory.Total,
			Unit:      "byte",
			Timestamp: now,
		},
	)

	temperature, ret := device.GetTemperature(
		nvml.TEMPERATURE_GPU,
	)
	if ret != nvml.SUCCESS {
		return nil, fmt.Errorf(
			"get temperature for NVIDIA device %q: %s",
			deviceID,
			nvml.ErrorString(ret),
		)
	}

	metrics = append(
		metrics,
		plugin.Metric{
			DeviceID:  deviceID,
			Name:      "temperature",
			Value:     float64(temperature),
			Unit:      "celsius",
			Timestamp: now,
		},
	)

	powerMilliwatts, ret := device.GetPowerUsage()
	if ret != nvml.SUCCESS {
		return nil, fmt.Errorf(
			"get power usage for NVIDIA device %q: %s",
			deviceID,
			nvml.ErrorString(ret),
		)
	}

	metrics = append(
		metrics,
		plugin.Metric{
			DeviceID: deviceID,
			Name:     "power",
			Value: float64(powerMilliwatts) /
				1000.0,
			Unit:      "watt",
			Timestamp: now,
		},
	)

	return metrics, nil
}
