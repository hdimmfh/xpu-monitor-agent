package profiler

import (
        "context"
        "time"
)

type Target struct {
        PID int `json:"pid"`

        DeviceID string `json:"device_id,omitempty"`

        Hostname string `json:"hostname,omitempty"`

        Command string `json:"command,omitempty"`

        ContainerID string `json:"container_id,omitempty"`

        JobID string `json:"job_id,omitempty"`
}

type Request struct {
        Target Target

        // Mode determines whether py-spy executes
        // dump or record.
        Mode string

        // The fields below are used only by record mode.
        Duration time.Duration

        SampleRate int

        Format string

        Native bool
}

type Profile struct {
        Profiler string `json:"profiler"`

        Mode string `json:"mode"`

        Target Target `json:"target"`

        StartedAt time.Time `json:"started_at"`

        EndedAt time.Time `json:"ended_at"`

        Format string `json:"format"`

        Data []byte `json:"data,omitempty"`

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
