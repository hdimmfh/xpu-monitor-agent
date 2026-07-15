package profiler

import (
	"context"
	"time"
)

// Target identifies the process and workload being profiled.
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

// Profile contains one profiling result.
//
// Data contains the profile itself in memory.
// It does not contain a file path.
type Profile struct {
	Profiler string `json:"profiler"`
	Target   Target `json:"target"`

	StartedAt time.Time `json:"started_at"`
	EndedAt   time.Time `json:"ended_at"`

	Format string `json:"format"`
	Data   []byte `json:"data"`

	Error string `json:"error,omitempty"`
}

// Text returns the in-memory profile as a string.
func (p Profile) Text() string {
	return string(p.Data)
}

// Profiler defines a workload profiler implementation.
type Profiler interface {
	Name() string

	Available(ctx context.Context) error

	// Profile performs one profiling operation and returns
	// the result directly in memory.
	Profile(
		ctx context.Context,
		request Request,
	) (Profile, error)
}
