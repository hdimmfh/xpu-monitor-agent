package profiler

import (
	"context"
	"encoding/json"
	"time"
)

// DetectedDevice represents a device that detected the target process.
//
// The structure is vendor-neutral. Vendor-specific process information
// can be stored in Metadata.
type DetectedDevice struct {
	// Plugin is the name of the plugin that detected the process.
	//
	// Examples:
	//   nvidia
	//   amd
	//   intel
	Plugin string `json:"plugin"`

	// ID uniquely identifies the device.
	//
	// For NVIDIA devices, this is normally the GPU UUID.
	ID string `json:"id"`

	// Vendor identifies the device vendor.
	Vendor string `json:"vendor"`

	// Model is the device model reported by the plugin.
	Model string `json:"model"`

	// Type describes the accelerator or device category.
	//
	// Examples:
	//   gpu
	//   xpu
	//   npu
	//   fpga
	//   asic
	Type string `json:"type"`

	// UsedMemoryBytes is the amount of device memory attributed to
	// the process.
	//
	// Zero means the vendor API did not report the value or the value
	// was unavailable.
	UsedMemoryBytes uint64 `json:"used_memory_bytes,omitempty"`

	// Metadata contains optional vendor-specific process information.
	//
	// NVIDIA examples:
	//   gpu_instance_id
	//   compute_instance_id
	Metadata map[string]string `json:"metadata,omitempty"`
}

// Target identifies the process being profiled and its execution context.
type Target struct {
	// PID is the host process ID.
	PID int `json:"pid"`

	// DeviceID is retained for backward compatibility.
	//
	// New device-aware profiling code should use Devices instead because
	// one process may be detected by multiple devices or multiple vendors.
	DeviceID string `json:"device_id,omitempty"`

	// Devices contains every device that detected the target process.
	//
	// A process using multiple accelerators is profiled once and all
	// associated devices are included here.
	Devices []DetectedDevice `json:"devices,omitempty"`

	// Hostname identifies the host on which the process is running.
	Hostname string `json:"hostname,omitempty"`

	// Command is the command associated with the target process.
	Command string `json:"command,omitempty"`

	// ContainerID identifies the container associated with the process.
	ContainerID string `json:"container_id,omitempty"`

	// JobID identifies the scheduler or workload job associated with
	// the process.
	JobID string `json:"job_id,omitempty"`
}

// Request contains the target and configuration for one profiling operation.
type Request struct {
	Target Target

	// Mode determines whether the profiler executes dump or record.
	Mode string

	// The fields below are used only by record mode.
	Duration   time.Duration
	SampleRate int
	Format     string

	// Native enables native stack collection when supported by the
	// profiler backend.
	Native bool
}

// Profile represents one profiling result.
type Profile struct {
	// Profiler identifies the profiler backend.
	Profiler string `json:"profiler"`

	// Mode is the profiling operation mode.
	Mode string `json:"mode"`

	// Target contains process and detected-device information.
	Target Target `json:"target"`

	// StartedAt is the time at which profiling began.
	StartedAt time.Time `json:"started_at"`

	// EndedAt is the time at which profiling completed.
	EndedAt time.Time `json:"ended_at"`

	// Format describes the original profiler output format.
	//
	// For py-spy dump, this remains "text" even though Data contains
	// the parsed JSON representation.
	Format string `json:"format"`

	// Data contains the structured JSON profiling result.
	//
	// json.RawMessage prevents the already encoded JSON document from
	// being converted into a Base64 string during marshaling.
	Data json.RawMessage `json:"data,omitempty"`

	// RawData preserves the original profiler output.
	//
	// It is intentionally excluded from the normal XPUMON JSON output.
	RawData []byte `json:"-"`

	// Error contains an error description when the profiling operation
	// failed.
	Error string `json:"error,omitempty"`
}

// Text returns the original profiler output when it is available.
//
// RawData is preferred because Data may contain a parsed JSON document.
// The Data fallback preserves compatibility with profiles constructed
// without RawData.
func (p Profile) Text() string {
	if len(p.RawData) > 0 {
		return string(p.RawData)
	}

	return string(p.Data)
}

// Profiler defines the common interface implemented by profiling backends.
type Profiler interface {
	Name() string

	// Available checks whether the underlying profiler binary can be
	// executed.
	Available(
		ctx context.Context,
	) error

	// Profile performs either a dump or record operation according to
	// Request.Mode.
	Profile(
		ctx context.Context,
		request Request,
	) (
		Profile,
		error,
	)
}
