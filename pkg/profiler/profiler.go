package profiler

import (
	"context"
	"time"
)

type Target struct {
	PID         int    `json:"pid"`
	DeviceID    string `json:"device_id,omitempty"`
	Hostname    string `json:"hostname,omitempty"`
	Command     string `json:"command,omitempty"`
	ContainerID string `json:"container_id,omitempty"`
	JobID       string `json:"job_id,omitempty"`
}

type Request struct {
	Target Target

	// Mode determines whether py-spy executes
	// dump or record.
	Mode string

	// The fields below are used only by record mode.
	Duration   time.Duration
	SampleRate int
	Format     string
	Native     bool
}

// StackSnapshot is a normalized stack snapshot.
//
// The py-spy-specific JSON response is converted
// into this common XPUMON model.
type StackSnapshot struct {
	Threads []StackThread `json:"threads"`
}

type StackThread struct {
	PID        int          `json:"pid"`
	ThreadID   uint64       `json:"thread_id"`
	ThreadName string       `json:"thread_name,omitempty"`
	OSThreadID uint64       `json:"os_thread_id"`
	Active     bool         `json:"active"`
	OwnsGIL    bool         `json:"owns_gil"`
	Frames     []StackFrame `json:"frames"`
}

type StackFrame struct {
	Name          string `json:"name"`
	Filename      string `json:"filename"`
	ShortFilename string `json:"short_filename,omitempty"`
	Module        string `json:"module,omitempty"`
	Line          int    `json:"line"`
}

type Profile struct {
	Profiler string `json:"profiler"`
	Mode     string `json:"mode"`

	Target Target `json:"target"`

	StartedAt time.Time `json:"started_at"`
	EndedAt   time.Time `json:"ended_at"`

	Format string `json:"format"`

	// Snapshot is populated only by dump mode.
	Snapshot *StackSnapshot `json:"snapshot,omitempty"`

	// Data preserves the original profiler payload.
	//
	// It is not directly serialized because encoding/json
	// serializes []byte as Base64.
	Data []byte `json:"-"`

	Error string `json:"error,omitempty"`
}

func (p Profile) Text() string {
	return string(p.Data)
}

type Profiler interface {
	Name() string

	// Available checks whether the underlying
	// profiler binary can be executed.
	Available(
		ctx context.Context,
	) error

	// Profile performs either a dump or record
	// operation according to Request.Mode.
	Profile(
		ctx context.Context,
		request Request,
	) (
		Profile,
		error,
	)
}
