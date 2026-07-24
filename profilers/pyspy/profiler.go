package pyspy

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	coreprofiler "github.com/hdimmfh/xpu-monitor-agent/pkg/profiler"
	coresource "github.com/hdimmfh/xpu-monitor-agent/pkg/source"
)

// Config contains settings required to execute py-spy.
type Config struct {
	BinaryPath string
}

// Profiler implements the common profiler interface using py-spy.
type Profiler struct {
	binaryPath     string
	sourceResolver coresource.Resolver
}

// New creates a py-spy profiler.
//
// When BinaryPath is empty, py-spy is resolved from PATH.
func New(
	cfg Config,
) (*Profiler, error) {
	binaryPath := strings.TrimSpace(cfg.BinaryPath)
	if binaryPath == "" {
		binaryPath = "py-spy"
	}

	return &Profiler{
		binaryPath:     binaryPath,
		sourceResolver: coresource.NewLinuxResolver(),
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

	path, err := exec.LookPath(p.binaryPath)
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
			strings.TrimSpace(string(output)),
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

	output, err := p.execute(ctx, request)
	if err != nil {
		return result, err
	}

	data, err := p.parseAndEnrichProfileData(
		ctx,
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

	// RawData preserves the original py-spy profile payload for debugging.
	// It must not be emitted in the normal JSON response.
	//
	// Dump mode stores py-spy stdout.
	// Record mode stores the contents of py-spy's output file.
	result.RawData = append(
		[]byte(nil),
		output...,
	)

	// Data contains valid JSON.
	result.Data = data

	return result, nil
}

// execute runs py-spy and returns only the profile payload.
//
// Dump mode writes its profile directly to stdout.
//
// Record mode writes progress messages to stdout/stderr and writes the actual
// profile to the path supplied through --output. Using a temporary file keeps
// diagnostic messages separate from raw, flamegraph, speedscope, and
// chrometrace profile data.
func (p *Profiler) execute(
	ctx context.Context,
	request coreprofiler.Request,
) ([]byte, error) {
	switch request.Mode {
	case coreprofiler.ModeDump:
		return p.executeDump(ctx, request)

	case coreprofiler.ModeRecord:
		return p.executeRecord(ctx, request)

	default:
		return nil, fmt.Errorf(
			"unsupported py-spy mode %q",
			request.Mode,
		)
	}
}

// executeDump runs py-spy dump and captures its stdout.
func (p *Profiler) executeDump(
	ctx context.Context,
	request coreprofiler.Request,
) ([]byte, error) {
	args := buildDumpArgs(request)

	cmd := exec.CommandContext(
		ctx,
		p.binaryPath,
		args...,
	)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	output, err := cmd.Output()
	if err != nil {
		return nil, pySpyExecutionError(
			ctx,
			request.Mode,
			err,
			stderr.String(),
		)
	}

	return output, nil
}

// executeRecord runs py-spy record with a temporary output file.
//
// The temporary file is required because using /dev/stdout causes py-spy's
// diagnostic messages and the actual profile payload to be mixed together.
func (p *Profiler) executeRecord(
	ctx context.Context,
	request coreprofiler.Request,
) ([]byte, error) {
	tempFile, err := os.CreateTemp(
		"",
		"xpumon-pyspy-record-*"+recordFileSuffix(request.Format),
	)
	if err != nil {
		return nil, fmt.Errorf(
			"create temporary py-spy record file: %w",
			err,
		)
	}

	tempPath := tempFile.Name()

	if err := tempFile.Close(); err != nil {
		_ = os.Remove(tempPath)

		return nil, fmt.Errorf(
			"close temporary py-spy record file %q: %w",
			tempPath,
			err,
		)
	}

	defer func() {
		_ = os.Remove(tempPath)
	}()

	args := buildRecordArgs(
		request,
		tempPath,
	)

	cmd := exec.CommandContext(
		ctx,
		p.binaryPath,
		args...,
	)

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	// py-spy can print progress or informational messages to stdout.
	// Capture them separately, but do not treat them as profile data.
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()
	if err != nil {
		diagnosticOutput := joinDiagnosticOutput(
			stderr.String(),
			stdout.String(),
		)

		return nil, pySpyExecutionError(
			ctx,
			request.Mode,
			err,
			diagnosticOutput,
		)
	}

	output, err := os.ReadFile(tempPath)
	if err != nil {
		return nil, fmt.Errorf(
			"read py-spy record output %q: %w",
			tempPath,
			err,
		)
	}

	if len(bytes.TrimSpace(output)) == 0 {
		diagnosticOutput := joinDiagnosticOutput(
			stderr.String(),
			stdout.String(),
		)

		if diagnosticOutput == "" {
			return nil, fmt.Errorf(
				"py-spy record produced an empty %s profile",
				request.Format,
			)
		}

		return nil, fmt.Errorf(
			"py-spy record produced an empty %s profile: %s",
			request.Format,
			diagnosticOutput,
		)
	}

	return output, nil
}

// pySpyExecutionError converts an exec error into a mode-specific error and
// preserves context cancellation and deadline errors.
func pySpyExecutionError(
	ctx context.Context,
	mode string,
	execErr error,
	diagnosticOutput string,
) error {
	switch {
	case errors.Is(ctx.Err(), context.Canceled):
		return fmt.Errorf(
			"py-spy %s canceled: %w",
			mode,
			ctx.Err(),
		)

	case errors.Is(ctx.Err(), context.DeadlineExceeded):
		return fmt.Errorf(
			"py-spy %s deadline exceeded: %w",
			mode,
			ctx.Err(),
		)
	}

	errorOutput := strings.TrimSpace(diagnosticOutput)
	if errorOutput == "" {
		errorOutput = execErr.Error()
	}

	return fmt.Errorf(
		"execute py-spy %s: %w: %s",
		mode,
		execErr,
		errorOutput,
	)
}

// joinDiagnosticOutput combines stderr and stdout diagnostic output without
// inserting unnecessary blank lines.
func joinDiagnosticOutput(
	stderr string,
	stdout string,
) string {
	parts := make([]string, 0, 2)

	if value := strings.TrimSpace(stderr); value != "" {
		parts = append(parts, value)
	}

	if value := strings.TrimSpace(stdout); value != "" {
		parts = append(parts, value)
	}

	return strings.Join(parts, "\n")
}

// parseAndEnrichProfileData converts py-spy output into JSON.
//
// Dump mode is parsed as DumpResult and enriched with source code from the
// target process filesystem.
//
// Record mode receives only the profile file contents and keeps the existing
// parser behavior.
func (p *Profiler) parseAndEnrichProfileData(
	ctx context.Context,
	request coreprofiler.Request,
	output []byte,
) ([]byte, error) {
	if request.Mode != coreprofiler.ModeDump {
		return parseProfileData(
			request,
			output,
		)
	}

	dump, err := parseDump(output)
	if err != nil {
		return nil, err
	}

	// Source resolution is best-effort. Individual files that cannot be
	// resolved do not invalidate the py-spy profile.
	//
	// EnrichSources only returns an error for invalid arguments or context
	// cancellation.
	if p.sourceResolver != nil {
		_, err = EnrichSources(
			ctx,
			p.sourceResolver,
			request.Target.PID,
			&dump,
		)
		if err != nil {
			return nil, fmt.Errorf(
				"enrich py-spy dump sources: %w",
				err,
			)
		}
	}

	data, err := json.Marshal(dump)
	if err != nil {
		return nil, fmt.Errorf(
			"marshal enriched py-spy dump: %w",
			err,
		)
	}

	return data, nil
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
		return validateRecordRequest(request)

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
	case "raw", "flamegraph", "speedscope", "chrometrace":
		return nil

	default:
		return fmt.Errorf(
			"unsupported py-spy format %q",
			request.Format,
		)
	}
}

func buildDumpArgs(
	request coreprofiler.Request,
) []string {
	args := []string{
		"dump",
		"--pid",
		strconv.Itoa(request.Target.PID),
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
	outputPath string,
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
		outputPath,
	}

	if request.Native {
		args = append(
			args,
			"--native",
		)
	}

	return args
}

// recordFileSuffix returns a conventional suffix for the py-spy output format.
//
// The suffix is not required by py-spy, but it makes temporary files easier
// to identify while debugging.
func recordFileSuffix(
	format string,
) string {
	switch format {
	case "raw":
		return ".txt"

	case "flamegraph":
		return ".svg"

	case "speedscope":
		return ".json"

	case "chrometrace":
		return ".json"

	default:
		return ".out"
	}
}

func resultFormat(
	request coreprofiler.Request,
) string {
	if request.Mode == coreprofiler.ModeDump {
		return "text"
	}

	return request.Format
}
