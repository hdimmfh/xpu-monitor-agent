package nvidia

import (
	"context"
	"fmt"
	"time"

	"github.com/NVIDIA/go-nvml/pkg/nvml"
	"github.com/hdimmfh/xpu-monitor-agent/pkg/plugin"
)

// Collect collects normalized telemetry from one NVIDIA GPU.
//
// Metrics unsupported by a specific NVIDIA device are skipped.
// Other NVML errors are returned to the caller.
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

	if err := collectUtilization(
		ctx,
		device,
		deviceID,
		now,
		&metrics,
	); err != nil {
		return nil, err
	}

	if err := collectMemory(
		ctx,
		device,
		deviceID,
		now,
		&metrics,
	); err != nil {
		return nil, err
	}

	if err := collectTemperature(
		ctx,
		device,
		deviceID,
		now,
		&metrics,
	); err != nil {
		return nil, err
	}

	if err := collectPower(
		ctx,
		device,
		deviceID,
		now,
		&metrics,
	); err != nil {
		return nil, err
	}

	return metrics, nil
}

func collectUtilization(
	ctx context.Context,
	device nvml.Device,
	deviceID string,
	timestamp time.Time,
	metrics *[]plugin.Metric,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	utilization, ret := device.GetUtilizationRates()

	switch ret {
	case nvml.SUCCESS:
		*metrics = append(
			*metrics,
			plugin.Metric{
				DeviceID:  deviceID,
				Name:      "gpu_utilization",
				Value:     float64(utilization.Gpu),
				Unit:      "percent",
				Timestamp: timestamp,
			},
			plugin.Metric{
				DeviceID:  deviceID,
				Name:      "memory_utilization",
				Value:     float64(utilization.Memory),
				Unit:      "percent",
				Timestamp: timestamp,
			},
		)

		return nil

	case nvml.ERROR_NOT_SUPPORTED:
		return nil

	default:
		return fmt.Errorf(
			"get utilization for NVIDIA device %q: %s",
			deviceID,
			nvml.ErrorString(ret),
		)
	}
}

func collectMemory(
	ctx context.Context,
	device nvml.Device,
	deviceID string,
	timestamp time.Time,
	metrics *[]plugin.Metric,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	memory, ret := device.GetMemoryInfo()

	switch ret {
	case nvml.SUCCESS:
		*metrics = append(
			*metrics,
			plugin.Metric{
				DeviceID:  deviceID,
				Name:      "memory_used",
				Value:     memory.Used,
				Unit:      "byte",
				Timestamp: timestamp,
			},
			plugin.Metric{
				DeviceID:  deviceID,
				Name:      "memory_total",
				Value:     memory.Total,
				Unit:      "byte",
				Timestamp: timestamp,
			},
		)

		return nil

	case nvml.ERROR_NOT_SUPPORTED:
		// UMA devices such as DGX Spark may not expose dedicated
		// GPU memory metrics through NVML.
		return nil

	default:
		return fmt.Errorf(
			"get memory info for NVIDIA device %q: %s",
			deviceID,
			nvml.ErrorString(ret),
		)
	}
}

func collectTemperature(
	ctx context.Context,
	device nvml.Device,
	deviceID string,
	timestamp time.Time,
	metrics *[]plugin.Metric,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	temperature, ret := device.GetTemperature(
		nvml.TEMPERATURE_GPU,
	)

	switch ret {
	case nvml.SUCCESS:
		*metrics = append(
			*metrics,
			plugin.Metric{
				DeviceID:  deviceID,
				Name:      "temperature",
				Value:     float64(temperature),
				Unit:      "celsius",
				Timestamp: timestamp,
			},
		)

		return nil

	case nvml.ERROR_NOT_SUPPORTED:
		return nil

	default:
		return fmt.Errorf(
			"get temperature for NVIDIA device %q: %s",
			deviceID,
			nvml.ErrorString(ret),
		)
	}
}

func collectPower(
	ctx context.Context,
	device nvml.Device,
	deviceID string,
	timestamp time.Time,
	metrics *[]plugin.Metric,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	powerMilliwatts, ret := device.GetPowerUsage()

	switch ret {
	case nvml.SUCCESS:
		*metrics = append(
			*metrics,
			plugin.Metric{
				DeviceID: deviceID,
				Name:     "power",
				Value: float64(powerMilliwatts) /
					1000.0,
				Unit:      "watt",
				Timestamp: timestamp,
			},
		)

		return nil

	case nvml.ERROR_NOT_SUPPORTED:
		return nil

	default:
		return fmt.Errorf(
			"get power usage for NVIDIA device %q: %s",
			deviceID,
			nvml.ErrorString(ret),
		)
	}
}
