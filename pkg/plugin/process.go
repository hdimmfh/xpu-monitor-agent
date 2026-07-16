package plugin

import "context"

// DeviceProcess represents a process detected by a device plugin.
type DeviceProcess struct {
	// PID is the host process ID.
	PID int `json:"pid"`

	// DeviceID identifies the device that detected the process.
	DeviceID string `json:"device_id"`

	// UsedMemoryBytes is the amount of device memory used by the process.
	//
	// Zero means the value is unavailable or was not reported by
	// the vendor API.
	UsedMemoryBytes uint64 `json:"used_memory_bytes,omitempty"`

	// Metadata contains vendor-specific information.
	//
	// Examples:
	//   NVIDIA:
	//     gpu_instance_id
	//     compute_instance_id
	//
	//   AMD:
	//     queue_id
	//
	//   Intel:
	//     engine_class
	Metadata map[string]string `json:"metadata,omitempty"`
}

// ProcessProvider is implemented by plugins capable of discovering
// processes currently using a device.
//
// Process discovery is optional and therefore intentionally separated
// from the core Plugin interface.
type ProcessProvider interface {
	Processes(
		ctx context.Context,
		deviceID string,
	) ([]DeviceProcess, error)
}
