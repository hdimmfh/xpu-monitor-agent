package pyspy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	coreprofiler "github.com/hdimmfh/xpu-monitor-agent/pkg/profiler"
)

type Config struct {
	BinaryPath string
	OutputDir  string
}

type Profiler struct {
	binaryPath string
	outputDir  string
}

func New(cfg Config) (*Profiler, error) {
	if strings.TrimSpace(cfg.BinaryPath) == "" {
		cfg.BinaryPath = "py-spy"
	}

	if strings.TrimSpace(cfg.OutputDir) == "" {
		cfg.OutputDir = "./profiles"
	}

	return &Profiler{
		binaryPath: cfg.BinaryPath,
		outputDir:  cfg.OutputDir,
	}, nil
}

func (p *Profiler) Name() string {
	return "py-spy"
}

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

func (p *Profiler) Profile(
	ctx context.Context,
	request coreprofiler.Request,
) (result coreprofiler.Result, returnErr error) {
	startedAt := time.Now().UTC()

	result = coreprofiler.Result{
		Profiler:  p.Name(),
		Target:    request.Target,
		StartedAt: startedAt,
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

	profileDir, err := p.createProfileDirectory(request, startedAt)
	if err != nil {
		return result, err
	}

	outputPath := filepath.Join(
		profileDir,
		"profile."+extensionForFormat(request.Format),
	)

	result.OutputPath = outputPath
	result.MetadataPath = filepath.Join(profileDir, "metadata.json")

	args := buildRecordArgs(request, outputPath)

	cmd := exec.CommandContext(ctx, p.binaryPath, args...)

	output, err := cmd.CombinedOutput()
	if err != nil {
		if errors.Is(ctx.Err(), context.Canceled) {
			return result, fmt.Errorf("py-spy profiling canceled: %w", ctx.Err())
		}

		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return result, fmt.Errorf(
				"py-spy profiling deadline exceeded: %w",
				ctx.Err(),
			)
		}

		return result, fmt.Errorf(
			"execute py-spy: %w: %s",
			err,
			strings.TrimSpace(string(output)),
		)
	}

	result.EndedAt = time.Now().UTC()

	if err := writeMetadata(result.MetadataPath, result); err != nil {
		return result, err
	}

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
		return fmt.Errorf("unsupported py-spy format %q", request.Format)
	}
}

func buildRecordArgs(
	request coreprofiler.Request,
	outputPath string,
) []string {
	// py-spy duration is passed in whole seconds.
	durationSeconds := int(math.Ceil(request.Duration.Seconds()))
	if durationSeconds < 1 {
		durationSeconds = 1
	}

	args := []string{
		"record",
		"--pid", strconv.Itoa(request.Target.PID),
		"--duration", strconv.Itoa(durationSeconds),
		"--rate", strconv.Itoa(request.SampleRate),
		"--format", request.Format,
		"--output", outputPath,
	}

	if request.Native {
		args = append(args, "--native")
	}

	return args
}

func (p *Profiler) createProfileDirectory(
	request coreprofiler.Request,
	startedAt time.Time,
) (string, error) {
	dateDirectory := startedAt.Format("2006-01-02")
	profileName := fmt.Sprintf(
		"pid-%d-%s",
		request.Target.PID,
		startedAt.Format("150405.000000000"),
	)

	path := filepath.Join(
		p.outputDir,
		dateDirectory,
		profileName,
	)

	if err := os.MkdirAll(path, 0o750); err != nil {
		return "", fmt.Errorf(
			"create profile directory %q: %w",
			path,
			err,
		)
	}

	return path, nil
}

func extensionForFormat(format string) string {
	switch format {
	case "flamegraph":
		return "svg"
	case "speedscope":
		return "json"
	case "chrometrace":
		return "json"
	case "raw":
		return "txt"
	default:
		return "out"
	}
}

func writeMetadata(
	path string,
	result coreprofiler.Result,
) error {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal profile metadata: %w", err)
	}

	data = append(data, '\n')

	if err := os.WriteFile(path, data, 0o640); err != nil {
		return fmt.Errorf(
			"write profile metadata %q: %w",
			path,
			err,
		)
	}

	return nil
}
