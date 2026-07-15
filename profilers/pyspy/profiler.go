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
func New(cfg Config) (*Profiler, error) {
	binaryPath := strings.TrimSpace(cfg.BinaryPath)
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
func (p *Profiler) Available(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	path, err := exec.LookPath(p.binaryPath)
	if err != nil {
		return fmt.Errorf(
			"find py-spy binary %q: %w",
			p.binaryPath,
			err,
		)
	}

	cmd := exec.CommandContext(ctx, path, "--version")

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf(
			"execute %q --version: %w: %s",
			path,
			err,
			strings.TrimSpace(string(output)),
		)
	}

	return nil
}

// Profile collects one profile and returns the result directly in memory.
//
// py-spy writes the profile payload to stdout through /dev/stdout.
// Nothing is persisted to a profile or metadata file.
func (p *Profiler) Profile(
	ctx context.Context,
	request coreprofiler.Request,
) (result coreprofiler.Profile, returnErr error) {
	result = coreprofiler.Profile{
		Profiler:  p.Name(),
		Target:    request.Target,
		StartedAt: time.Now().UTC(),
		Format:    request.Format,
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

	args := buildRecordArgs(request)

	cmd := exec.CommandContext(
		ctx,
		p.binaryPath,
		args...,
	)

	// Profile data and diagnostic messages must be separated.
	//
	// stdout: py-spy profile payload
	// stderr: py-spy progress and error messages
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	output, err := cmd.Output()
	if err != nil {
		switch {
		case errors.Is(ctx.Err(), context.Canceled):
			return result, fmt.Errorf(
				"py-spy profiling canceled: %w",
				ctx.Err(),
			)

		case errors.Is(ctx.Err(), context.DeadlineExceeded):
			return result, fmt.Errorf(
				"py-spy profiling deadline exceeded: %w",
				ctx.Err(),
			)
		}

		errorOutput := strings.TrimSpace(stderr.String())
		if errorOutput == "" {
			errorOutput = err.Error()
		}

		return result, fmt.Errorf(
			"execute py-spy: %w: %s",
			err,
			errorOutput,
		)
	}

	result.Data = output

	return result, nil
}

func validateRequest(request coreprofiler.Request) error {
	if request.Target.PID <= 0 {
		return fmt.Errorf("PID must be greater than zero")
	}

	if request.Duration <= 0 {
		return fmt.Errorf("duration must be greater than zero")
	}

	if request.SampleRate <= 0 {
		return fmt.Errorf("sample rate must be greater than zero")
	}

	switch request.Format {
	case "raw", "flamegraph", "speedscope", "chrometrace":
		return nil

	default:
		return fmt.Errorf(
			"unsupported py-spy format %q",
			request.Format,
		)
	}
}

func buildRecordArgs(
	request coreprofiler.Request,
) []string {
	// py-spy accepts duration as whole seconds.
	durationSeconds := int(
		math.Ceil(request.Duration.Seconds()),
	)
	if durationSeconds < 1 {
		durationSeconds = 1
	}

	args := []string{
		"record",
		"--pid",
		strconv.Itoa(request.Target.PID),
		"--duration",
		strconv.Itoa(durationSeconds),
		"--rate",
		strconv.Itoa(request.SampleRate),
		"--format",
		request.Format,
		"--output",
		"/dev/stdout",
	}

	if request.Native {
		args = append(args, "--native")
	}

	return args
}
