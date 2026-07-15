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

	if err := run(ctx, os.Args[1:]); err != nil {
		log.Fatal(err)
	}
}

func run(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return runCollect(ctx)
	}

	switch args[0] {
	case "collect":
		return runCollect(ctx)

	case "profile":
		return runProfile(ctx, args[1:])

	case "help", "-h", "--help":
		printUsage()
		return nil

	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

// runCollect collects host and NVIDIA device metrics.
//
// The existing metric output format is intentionally preserved.
func runCollect(ctx context.Context) error {
	nvidiaPlugin, err := nvidia.New()
	if err != nil {
		return fmt.Errorf("create NVIDIA plugin: %w", err)
	}

	defer func() {
		if err := nvidiaPlugin.Close(); err != nil {
			log.Printf("close NVIDIA plugin: %v", err)
		}
	}()

	hostPlugin := host.New()

	c := collector.New(
		hostPlugin,
		nvidiaPlugin,
	)

	metrics, err := c.CollectAll(ctx)
	if err != nil {
		return fmt.Errorf("collect metrics: %w", err)
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

// runProfile performs one py-spy profiling operation.
//
// The profile is returned directly in memory through Profile.Data.
// No profile or metadata file is created.
func runProfile(ctx context.Context, args []string) error {
	flags := flag.NewFlagSet(
		"profile",
		flag.ContinueOnError,
	)

	configPath := flags.String(
		"config",
		"./configs/xpumon.yaml",
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
		"profiling duration override, for example 10s",
	)

	rateOverride := flags.Int(
		"rate",
		0,
		"sampling rate override",
	)

	formatOverride := flags.String(
		"format",
		"",
		"output format override: raw, flamegraph, speedscope, or chrometrace",
	)

	nativeOverride := flags.Bool(
		"native",
		false,
		"include native stack frames",
	)

	withMetrics := flags.Bool(
		"with-metrics",
		false,
		"collect host and device metrics before profiling",
	)

	if err := flags.Parse(args); err != nil {
		return err
	}

	if *pid <= 0 {
		return errors.New("--pid must be greater than zero")
	}

	cfg, err := coreprofiler.LoadConfig(*configPath)
	if err != nil {
		return err
	}

	if !cfg.Profiling.Enabled {
		return errors.New("profiling is disabled in configuration")
	}

	duration, err := cfg.Duration()
	if err != nil {
		return fmt.Errorf("parse profiling duration: %w", err)
	}

	if *durationOverride > 0 {
		duration = *durationOverride
	}

	sampleRate := cfg.Profiling.PySpy.SampleRate
	if *rateOverride > 0 {
		sampleRate = *rateOverride
	}

	format := cfg.Profiling.PySpy.Format
	if *formatOverride != "" {
		format = *formatOverride
	}

	native := cfg.Profiling.PySpy.Native || *nativeOverride

	hostname, err := os.Hostname()
	if err != nil {
		return fmt.Errorf("get hostname: %w", err)
	}

	p, err := pyspy.New(pyspy.Config{
		BinaryPath: cfg.Profiling.PySpy.Binary,
	})
	if err != nil {
		return fmt.Errorf("create py-spy profiler: %w", err)
	}

	if *withMetrics {
		if err := runCollect(ctx); err != nil {
			return err
		}

		fmt.Println()
	}

	result, err := p.Profile(
		ctx,
		coreprofiler.Request{
			Target: coreprofiler.Target{
				PID:         *pid,
				DeviceID:    *deviceID,
				Hostname:    hostname,
				Command:     *command,
				ContainerID: *containerID,
				JobID:       *jobID,
			},
			Duration:   duration,
			SampleRate: sampleRate,
			Format:     format,
			Native:     native,
		},
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

func printProfile(profile coreprofiler.Profile) {
	fmt.Printf(
		"profile=%s pid=%d format=%s started_at=%s ended_at=%s",
		profile.Profiler,
		profile.Target.PID,
		profile.Format,
		profile.StartedAt.Format(time.RFC3339Nano),
		profile.EndedAt.Format(time.RFC3339Nano),
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
	fmt.Println("profile_data_begin")

	fmt.Print(profile.Text())

	if len(profile.Data) > 0 &&
		profile.Data[len(profile.Data)-1] != '\n' {
		fmt.Println()
	}

	fmt.Println("profile_data_end")
}

func printUsage() {
	fmt.Print(`XPUMON

Usage:
  xpumon
  xpumon collect
  xpumon profile --pid <PID> [options]

Commands:
  collect
      Collect host and accelerator metrics.

  profile
      Collect one py-spy profile and return it as text.
      No profile file is created.

Profile options:
  --config <path>
      Configuration file path.
      Default: ./configs/xpumon.yaml

  --pid <PID>
      Target Python process ID. Required.

  --duration <duration>
      Override the YAML profiling duration.
      Example: 10s

  --rate <number>
      Override the YAML sampling rate.

  --format <format>
      Override the YAML output format.
      Supported: raw, flamegraph, speedscope, chrometrace

  --native
      Include native stack frames.

  --with-metrics
      Print existing host and device metrics before the profile.

  --device-id <ID>
      Device ID associated with the process.

  --container-id <ID>
      Container ID associated with the process.

  --job-id <ID>
      Job ID associated with the process.

  --command <command>
      Command associated with the process.

Examples:
  xpumon
  xpumon collect

  xpumon profile \
    --pid 124243

  xpumon profile \
    --pid 124243 \
    --duration 10s \
    --rate 20 \
    --format raw

  xpumon profile \
    --pid 124243 \
    --with-metrics \
    --device-id GPU-8994d77e-21c7-99de-5b47-d8180c8d8623
`)
}
