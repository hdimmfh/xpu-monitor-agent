package profiler

import (
	"context"
	"time"
)

type Target struct {
	PID         int    `json:"pid"`
	DeviceID    string `json:"device_id,omitempty"`
	Command     string `json:"command,omitempty"`
	ContainerID string `json:"container_id,omitempty"`
	JobID       string `json:"job_id,omitempty"`
}

type Request struct {
	Target     Target
	Duration   time.Duration
	SampleRate int
	Format     string
	Native     bool
}

type Result struct {
	Profiler  string    `json:"profiler"`
	Target    Target    `json:"target"`
	StartedAt time.Time `json:"started_at"`
	EndedAt   time.Time `json:"ended_at"`

	Format     string `json:"format"`
	OutputPath string `json:"output_path"`
	MetadataPath string `json:"metadata_path"`

	Error string `json:"error,omitempty"`
}

type Profiler interface {
	Name() string

	// Available checks whether the underlying profiler binary
	// can be executed.
	Available(ctx context.Context) error

	// Profile collects a profile for the requested target.
	Profile(ctx context.Context, request Request) (Result, error)
}
