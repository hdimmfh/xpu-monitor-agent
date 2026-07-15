package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/hdimmfh/xpu-monitor-agent/pkg/collector"
	coreprofiler "github.com/hdimmfh/xpu-monitor-agent/pkg/profiler"
	"github.com/hdimmfh/xpu-monitor-agent/plugins/host"
	"github.com/hdimmfh/xpu-monitor-agent/plugins/nvidia"
	"github.com/hdimmfh/xpu-monitor-agent/profilers/pyspy"
)

func main() {
	ctx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stop()

	if err := run(
		ctx,
		os.Args[1:],
	); err != nil {
		log.Fatal(err)
	}
}

func run(
	ctx context.Context,
	args []string,
) error {
	if len(args) == 0 {
		return runCollect(ctx)
	}

	switch args[0] {
	case "collect":
		return runCollect(ctx)

	case "profile":
		return runProfile(
			ctx,
			args[1:],
		)

	case "help", "-h", "--help":
		printUsage()

		return nil

	default:
		return fmt.Errorf(
			"unknown command %q",
			args[0],
		)
	}
}

// runCollect collects host and NVIDIA
// device metrics.
func runCollect(
	ctx context.Context,
) error {
	nvidiaPlugin, err := nvidia.New()
	if err != nil {
		return fmt.Errorf(
			"create NVIDIA plugin: %w",
			err,
		)
	}

	defer func() {
		if err := nvidiaPlugin.Close(); err != nil {
			log.Printf(
				"close NVIDIA plugin: %v",
				err,
			)
		}
	}()

	hostPlugin := host.New()

	c := collector.New(
		hostPlugin,
		nvidiaPlugin,
	)

	metrics, err := c.CollectAll(ctx)
	if err != nil {
		return fmt.Errorf(
			"collect metrics: %w",
			err,
		)
	}

	for _, metric := range metrics {
		fmt.Printf(
			"device=%s metric=%s value=%v unit=%s\n",
			metric.DeviceID,
			metric.Name,
			metric.Value,
			metric.Unit,
		)
	}

	return nil
}

// runProfile performs either a py-spy dump
// or record operation according to the YAML mode.
func runProfile(
	ctx context.Context,
	args []string,
) error {
	flags := flag.NewFlagSet(
		"profile",
		flag.ContinueOnError,
	)

	configPath := flags.String(
		"config",
		"./configs/pyspy-dump.yaml",
		"path to XPUMON configuration file",
	)

	pid := flags.Int(
		"pid",
		0,
		"target Python process ID",
	)

	deviceID := flags.String(
		"device-id",
		"",
		"device ID associated with the target process",
	)

	containerID := flags.String(
		"container-id",
		"",
		"container ID associated with the target process",
	)

	jobID := flags.String(
		"job-id",
		"",
		"job ID associated with the target process",
	)

	command := flags.String(
		"command",
		"",
		"command associated with the target process",
	)

	durationOverride := flags.Duration(
		"duration",
		0,
		"record duration override",
	)

	rateOverride := flags.Int(
		"rate",
		0,
		"record sampling rate override",
	)

	formatOverride := flags.String(
		"format",
		"",
		"record output format override",
	)

	nativeOverride := flags.Bool(
		"native",
		false,
		"include native stack frames",
	)

	withMetrics := flags.Bool(
		"with-metrics",
		false,
		"collect metrics before profiling",
	)

	if err := flags.Parse(args); err != nil {
		return err
	}

	if *pid <= 0 {
		return errors.New(
			"--pid must be greater than zero",
		)
	}

	cfg, err := coreprofiler.LoadConfig(
		*configPath,
	)
	if err != nil {
		return err
	}

	if !cfg.Profiling.Enabled {
		return errors.New(
			"profiling is disabled in configuration",
		)
	}

	request, err := buildProfileRequest(
		cfg,
		*pid,
		*deviceID,
		*containerID,
		*jobID,
		*command,
		*durationOverride,
		*rateOverride,
		*formatOverride,
		*nativeOverride,
	)
	if err != nil {
		return err
	}

	p, err := pyspy.New(
		pyspy.Config{
			BinaryPath:
				cfg.Profiling.PySpy.Binary,
		},
	)
	if err != nil {
		return fmt.Errorf(
			"create py-spy profiler: %w",
			err,
		)
	}

	if *withMetrics {
		if err := runCollect(ctx); err != nil {
			return err
		}

		fmt.Println()
	}

	result, err := p.Profile(
		ctx,
		request,
	)
	if err != nil {
		return fmt.Errorf(
			"profile PID %d: %w",
			*pid,
			err,
		)
	}

	printProfile(result)

	return nil
}

func buildProfileRequest(
	cfg coreprofiler.Config,
	pid int,
	deviceID string,
	containerID string,
	jobID string,
	command string,
	durationOverride time.Duration,
	rateOverride int,
	formatOverride string,
	nativeOverride bool,
) (
	coreprofiler.Request,
	error,
) {
	hostname, err := os.Hostname()
	if err != nil {
		return coreprofiler.Request{},
			fmt.Errorf(
				"get hostname: %w",
				err,
			)
	}

	request := coreprofiler.Request{
		Mode: cfg.Profiling.PySpy.Mode,

		Target: coreprofiler.Target{
			PID:         pid,
			DeviceID:    deviceID,
			Hostname:    hostname,
			Command:     command,
			ContainerID: containerID,
			JobID:       jobID,
		},

		Native:
			cfg.Profiling.PySpy.Native ||
				nativeOverride,
	}

	if request.Mode ==
		coreprofiler.ModeDump {
		return request, nil
	}

	duration, err := cfg.Duration()
	if err != nil {
		return coreprofiler.Request{},
			fmt.Errorf(
				"parse profiling duration: %w",
				err,
			)
	}

	if durationOverride > 0 {
		duration = durationOverride
	}

	sampleRate :=
		cfg.Profiling.PySpy.SampleRate

	if rateOverride > 0 {
		sampleRate = rateOverride
	}

	format :=
		cfg.Profiling.PySpy.Format

	if formatOverride != "" {
		format = formatOverride
	}

	request.Duration = duration
	request.SampleRate = sampleRate
	request.Format = format

	return request, nil
}

func printProfile(
	profile coreprofiler.Profile,
) {
	fmt.Printf(
		"profile=%s mode=%s pid=%d format=%s started_at=%s ended_at=%s",
		profile.Profiler,
		profile.Mode,
		profile.Target.PID,
		profile.Format,
		profile.StartedAt.Format(
			time.RFC3339Nano,
		),
		profile.EndedAt.Format(
			time.RFC3339Nano,
		),
	)

	if profile.Target.Hostname != "" {
		fmt.Printf(
			" hostname=%q",
			profile.Target.Hostname,
		)
	}

	if profile.Target.DeviceID != "" {
		fmt.Printf(
			" device=%q",
			profile.Target.DeviceID,
		)
	}

	if profile.Target.ContainerID != "" {
		fmt.Printf(
			" container_id=%q",
			profile.Target.ContainerID,
		)
	}

	if profile.Target.JobID != "" {
		fmt.Printf(
			" job_id=%q",
			profile.Target.JobID,
		)
	}

	if profile.Target.Command != "" {
		fmt.Printf(
			" command=%q",
			profile.Target.Command,
		)
	}

	fmt.Println()

	fmt.Println(
		"profile_data_begin",
	)

	fmt.Print(
		profile.Text(),
	)

	if len(profile.Data) > 0 &&
		profile.Data[
			len(profile.Data)-1
		] != '\n' {
		fmt.Println()
	}

	fmt.Println(
		"profile_data_end",
	)
}

func printUsage() {
	fmt.Print(
		`XPUMON

Usage:

  xpumon
  xpumon collect

  xpumon profile \
      --config <config.yaml> \
      --pid <PID>

YAML modes:

  mode: dump

      Execute:

          py-spy dump --pid <PID>

      Captures one current stack snapshot.

  mode: record

      Execute:

          py-spy record \
              --pid <PID> \
              --duration <seconds> \
              --rate <rate> \
              --format <format>

      Samples stacks during the configured duration.

Options:

  --config <path>

      YAML configuration file.

  --pid <PID>

      Target Python process ID.

  --duration <duration>

      Override record duration.

      Ignored in dump mode.

  --rate <rate>

      Override record sampling rate.

      Ignored in dump mode.

  --format <format>

      Override record output format.

      Ignored in dump mode.

  --native

      Include native stack frames.

  --with-metrics

      Collect host and accelerator
      metrics before profiling.

Examples:

  go run ./cmd/xpumon \
      profile \
      --config ./configs/pyspy-dump.yaml \
      --pid 1234

  go run ./cmd/xpumon \
      profile \
      --config ./configs/pyspy-record.yaml \
      --pid 1234

`,
	)
}
