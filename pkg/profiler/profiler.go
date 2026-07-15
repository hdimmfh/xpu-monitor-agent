package profiler

import (
	"context"
	"time"
)

// Target identifies the process and workload being profiled.
//
// PID alone is not globally unique because the same PID can exist
// on different hosts or in different PID namespaces. Device, host,
// container, and job metadata are therefore retained separately.
type Target struct {
	PID int `json:"pid"`

	DeviceID string `json:"device_id,omitempty"`
	Hostname string `json:"hostname,omitempty"`

	Command     string `json:"command,omitempty"`
	ContainerID string `json:"container_id,omitempty"`
	JobID       string `json:"job_id,omitempty"`
}

// Request describes a single profiling operation.
type Request struct {
	Target Target

	Duration   time.Duration
	SampleRate int
	Format     string
	Native     bool
}

// Profile contains the result of one profiling operation.
//
// Data holds the profiling result in memory. It is not a path and does
// not imply persistence. For py-spy raw format, Data contains the same
// folded stack text that was previously written to profile.txt.
type Profile struct {
	Profiler string `json:"profiler"`
	Target   Target `json:"target"`

	StartedAt time.Time `json:"started_at"`
	EndedAt   time.Time `json:"ended_at"`

	Format string `json:"format"`
	Data   []byte `json:"data"`

	Error string `json:"error,omitempty"`
}

// Text returns the profile payload as text.
//
// This is primarily useful for text-based formats such as py-spy raw.
// Binary or structured formats should generally consume Data directly.
func (p Profile) Text() string {
	return string(p.Data)
}

// Profiler defines a workload profiler implementation.
//
// Implementations collect and return exactly one profiling result for
// each Profile call. Retention or history management belongs outside
// this interface.
type Profiler interface {
	Name() string

	// Available checks whether the underlying profiler implementation
	// can be executed in the current environment.
	Available(ctx context.Context) error

	// Profile collects one profile for the requested target and returns
	// its content directly in memory.
	Profile(ctx context.Context, request Request) (Profile, error)
}
