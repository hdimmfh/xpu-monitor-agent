package pyspy

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math"
	"os/exec"
	"strconv"
	"strings"
	"time"

	coreprofiler "github.com/hdimmfh/xpu-monitor-agent/pkg/profiler"
)

// Config contains settings required to execute py-spy.
type Config struct {
	BinaryPath string
}

// Profiler implements the common profiler interface using py-spy.
type Profiler struct {
	binaryPath string
}

// New creates a py-spy profiler.
//
// When BinaryPath is empty, py-spy is resolved from PATH.
func New(
	cfg Config,
) (*Profiler, error) {
	binaryPath := strings.TrimSpace(
		cfg.BinaryPath,
	)

	if binaryPath == "" {
		binaryPath = "py-spy"
	}

	return &Profiler{
		binaryPath: binaryPath,
	}, nil
}

// Name returns the profiler implementation name.
func (p *Profiler) Name() string {
	return "py-spy"
}

// Available verifies that the py-spy binary can be found and executed.
func (p *Profiler) Available(
	ctx context.Context,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	path, err := exec.LookPath(
		p.binaryPath,
	)
	if err != nil {
		return fmt.Errorf(
			"find py-spy binary %q: %w",
			p.binaryPath,
			err,
		)
	}

	cmd := exec.CommandContext(
		ctx,
		path,
		"--version",
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf(
			"execute %q --version: %w: %s",
			path,
			err,
			strings.TrimSpace(
				string(output),
			),
		)
	}

	return nil
}

// Profile executes either py-spy dump or py-spy record according to
// request.Mode.
func (p *Profiler) Profile(
	ctx context.Context,
	request coreprofiler.Request,
) (
	result coreprofiler.Profile,
	returnErr error,
) {
	result = coreprofiler.Profile{
		Profiler:  p.Name(),
		Mode:      request.Mode,
		Target:    request.Target,
		StartedAt: time.Now().UTC(),
		Format:    resultFormat(request),
	}

	defer func() {
		result.EndedAt = time.Now().UTC()

		if returnErr != nil {
			result.Error = returnErr.Error()
		}
	}()

	if err := validateRequest(request); err != nil {
		return result, err
	}

	if err := p.Available(ctx); err != nil {
		return result, err
	}

	args, err := buildArgs(request)
	if err != nil {
		return result, err
	}

	cmd := exec.CommandContext(
		ctx,
		p.binaryPath,
		args...,
	)

	// stdout contains the dump output or record payload.
	// stderr contains py-spy diagnostic and progress messages.
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	output, err := cmd.Output()
	if err != nil {
		switch {
		case errors.Is(
			ctx.Err(),
			context.Canceled,
		):
			return result, fmt.Errorf(
				"py-spy %s canceled: %w",
				request.Mode,
				ctx.Err(),
			)

		case errors.Is(
			ctx.Err(),
			context.DeadlineExceeded,
		):
			return result, fmt.Errorf(
				"py-spy %s deadline exceeded: %w",
				request.Mode,
				ctx.Err(),
			)
		}

		errorOutput := strings.TrimSpace(
			stderr.String(),
		)

		if errorOutput == "" {
			errorOutput = err.Error()
		}

		return result, fmt.Errorf(
			"execute py-spy %s: %w: %s",
			request.Mode,
			err,
			errorOutput,
		)
	}

	data, err := parseProfileData(
		request,
		output,
	)
	if err != nil {
		return result, fmt.Errorf(
			"parse py-spy %s output: %w",
			request.Mode,
			err,
		)
	}

	// RawData preserves the original py-spy stdout for debugging.
	// It must not be emitted in the normal JSON response.
	result.RawData = append(
		[]byte(nil),
		output...,
	)

	// Data contains valid JSON.
	result.Data = data

	return result, nil
}

func validateRequest(
	request coreprofiler.Request,
) error {
	if request.Target.PID <= 0 {
		return fmt.Errorf(
			"PID must be greater than zero",
		)
	}

	switch request.Mode {
	case coreprofiler.ModeDump:
		return nil

	case coreprofiler.ModeRecord:
		return validateRecordRequest(
			request,
		)

	default:
		return fmt.Errorf(
			"unsupported py-spy mode %q",
			request.Mode,
		)
	}
}

func validateRecordRequest(
	request coreprofiler.Request,
) error {
	if request.Duration <= 0 {
		return fmt.Errorf(
			"duration must be greater than zero",
		)
	}

	if request.SampleRate <= 0 {
		return fmt.Errorf(
			"sample rate must be greater than zero",
		)
	}

	switch request.Format {
	case "raw",
		"flamegraph",
		"speedscope",
		"chrometrace":
		return nil

	default:
		return fmt.Errorf(
			"unsupported py-spy format %q",
			request.Format,
		)
	}
}

func buildArgs(
	request coreprofiler.Request,
) ([]string, error) {
	switch request.Mode {
	case coreprofiler.ModeDump:
		return buildDumpArgs(
			request,
		), nil

	case coreprofiler.ModeRecord:
		return buildRecordArgs(
			request,
		), nil

	default:
		return nil, fmt.Errorf(
			"unsupported py-spy mode %q",
			request.Mode,
		)
	}
}

func buildDumpArgs(
	request coreprofiler.Request,
) []string {
	args := []string{
		"dump",
		"--pid",
		strconv.Itoa(
			request.Target.PID,
		),
	}

	if request.Native {
		args = append(
			args,
			"--native",
		)
	}

	return args
}

func buildRecordArgs(
	request coreprofiler.Request,
) []string {
	// py-spy accepts duration as whole seconds.
	durationSeconds := int(
		math.Ceil(
			request.Duration.Seconds(),
		),
	)

	if durationSeconds < 1 {
		durationSeconds = 1
	}

	args := []string{
		"record",
		"--pid",
		strconv.Itoa(
			request.Target.PID,
		),
		"--duration",
		strconv.Itoa(
			durationSeconds,
		),
		"--rate",
		strconv.Itoa(
			request.SampleRate,
		),
		"--format",
		request.Format,
		"--output",
		"/dev/stdout",
	}

	if request.Native {
		args = append(
			args,
			"--native",
		)
	}

	return args
}

func resultFormat(
	request coreprofiler.Request,
) string {
	if request.Mode == coreprofiler.ModeDump {
		return "text"
	}

	return request.Format
}
