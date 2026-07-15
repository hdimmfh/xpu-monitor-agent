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
	"github.com/hdimmfh/xpu-monitor-agent/pkg/plugin"
	coreprofiler "github.com/hdimmfh/xpu-monitor-agent/pkg/profiler"
	"github.com/hdimmfh/xpu-monitor-agent/plugins/host"
	"github.com/hdimmfh/xpu-monitor-agent/plugins/nvidia"
	"github.com/hdimmfh/xpu-monitor-agent/profilers/pyspy"
	"github.com/hdimmfh/xpu-monitor-agent/pkg/process"
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
	// The host plugin is always available and registered first.
	plugins := []plugin.Plugin{
		host.New(),
	}

	// NVIDIA is optional.
	//
	// If NVML is unavailable, continue collecting host metrics.
	nvidiaPlugin, err := nvidia.New()
	if err != nil {
		log.Printf(
			"NVIDIA plugin unavailable; continuing with host metrics: %v",
			err,
		)
	} else {
		plugins = append(
			plugins,
			nvidiaPlugin,
		)

		defer func() {
			if err := nvidiaPlugin.Close(); err != nil {
				log.Printf(
					"close NVIDIA plugin: %v",
					err,
				)
			}
		}()
	}

	c := collector.New(
		plugins...,
	)

	metrics, collectErr := c.CollectAll(ctx)

	// Print successfully collected metrics even if another
	// plugin failed.
	for _, metric := range metrics {
		fmt.Printf(
			"device=%s metric=%s value=%v unit=%s\n",
			metric.DeviceID,
			metric.Name,
			metric.Value,
			metric.Unit,
		)
	}

	if collectErr != nil {
		// No plugin produced metrics.
		if len(metrics) == 0 {
			return fmt.Errorf(
				"collect metrics: %w",
				collectErr,
			)
		}

		// At least one plugin succeeded.
		//
		// Keep running and report the failed plugin as a warning.
		log.Printf(
			"partial metric collection failure: %v",
			collectErr,
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
		"optional target Python process ID; omit to discover all Python processes",
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

	if *pid < 0 {
		return fmt.Errorf(
			"--pid must not be negative",
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
	
	if *pid > 0 {
		return profileSingleProcess(
			ctx,
			p,
			cfg,
			process.Process{
				PID:     *pid,
				Command: *command,
			},
			*deviceID,
			*containerID,
			*jobID,
			*durationOverride,
			*rateOverride,
			*formatOverride,
			*nativeOverride,
		)
	}
	
	if !cfg.Profiling.Discovery.Enabled {
		return fmt.Errorf(
			"--pid is required when profiling.discovery.enabled is false",
		)
	}
	
	return profileDiscoveredProcesses(
		ctx,
		p,
		cfg,
		*deviceID,
		*jobID,
		*durationOverride,
		*rateOverride,
		*formatOverride,
		*nativeOverride,
	)
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
	elapsed := profile.EndedAt.Sub(profile.StartedAt)

	fmt.Println("================================================================================")
	fmt.Println("PY-SPY PROFILE")
	fmt.Println("================================================================================")

	fmt.Printf("Profiler   : %s\n", profile.Profiler)
	fmt.Printf("PID        : %d\n", profile.Target.PID)
	fmt.Printf("Format     : %s\n", profile.Format)
	fmt.Printf(
		"Started    : %s\n",
		profile.StartedAt.UTC().Format("2006-01-02 15:04:05.000 MST"),
	)
	fmt.Printf(
		"Ended      : %s\n",
		profile.EndedAt.UTC().Format("2006-01-02 15:04:05.000 MST"),
	)
	fmt.Printf(
		"Elapsed    : %s\n",
		elapsed.Round(time.Microsecond),
	)

	if profile.Target.Hostname != "" {
		fmt.Printf(
			"Hostname   : %s\n",
			profile.Target.Hostname,
		)
	}

	if profile.Target.DeviceID != "" {
		fmt.Printf(
			"Device     : %s\n",
			profile.Target.DeviceID,
		)
	}

	if profile.Target.ContainerID != "" {
		fmt.Printf(
			"Container  : %s\n",
			profile.Target.ContainerID,
		)
	}

	if profile.Target.JobID != "" {
		fmt.Printf(
			"Job ID     : %s\n",
			profile.Target.JobID,
		)
	}

	if profile.Target.Command != "" {
		fmt.Printf(
			"Command    : %s\n",
			profile.Target.Command,
		)
	}

	fmt.Println("--------------------------------------------------------------------------------")

	fmt.Print(
		profile.Text(),
	)

	if len(profile.Data) > 0 &&
		profile.Data[len(profile.Data)-1] != '\n' {
		fmt.Println()
	}

	fmt.Println("================================================================================")
	fmt.Println()
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

func profileDiscoveredProcesses(
	ctx context.Context,
	p coreprofiler.Profiler,
	cfg coreprofiler.Config,
	deviceID string,
	jobID string,
	durationOverride time.Duration,
	rateOverride int,
	formatOverride string,
	nativeOverride bool,
) error {
	processes, err := process.DiscoverPython(
		cfg.Profiling.Discovery.ProcRoot,
	)
	if err != nil {
		return fmt.Errorf(
			"discover Python processes: %w",
			err,
		)
	}

	filter, err := process.NewFilter(
		cfg.Profiling.Discovery.Exclude,
	)
	if err != nil {
		return fmt.Errorf(
			"create process filter: %w",
			err,
		)
	}

	profiledCount := 0
	excludedCount := 0
	failedCount := 0

	for _, targetProcess := range processes {
		if err := ctx.Err(); err != nil {
			return err
		}

		excluded, reason := filter.Excluded(
			targetProcess,
		)
		if excluded {
			excludedCount++

			log.Printf(
				"skip Python process pid=%d command=%q reason=%s",
				targetProcess.PID,
				targetProcess.Command,
				reason,
			)

			continue
		}

		err := profileSingleProcess(
			ctx,
			p,
			cfg,
			targetProcess,
			deviceID,
			"",
			jobID,
			durationOverride,
			rateOverride,
			formatOverride,
			nativeOverride,
		)
		if err != nil {
			failedCount++

			log.Printf(
				"profile Python process pid=%d command=%q: %v",
				targetProcess.PID,
				targetProcess.Command,
				err,
			)

			continue
		}

		profiledCount++
	}

	log.Printf(
		"Python profiling complete discovered=%d profiled=%d excluded=%d failed=%d",
		len(processes),
		profiledCount,
		excludedCount,
		failedCount,
	)

	if profiledCount == 0 && failedCount > 0 {
		return fmt.Errorf(
			"all selected Python processes failed profiling",
		)
	}

	return nil
}

func profileSingleProcess(
	ctx context.Context,
	p coreprofiler.Profiler,
	cfg coreprofiler.Config,
	targetProcess process.Process,
	deviceID string,
	containerID string,
	jobID string,
	durationOverride time.Duration,
	rateOverride int,
	formatOverride string,
	nativeOverride bool,
) error {
	request, err := buildProfileRequest(
		cfg,
		targetProcess.PID,
		deviceID,
		containerID,
		jobID,
		targetProcess.Command,
		durationOverride,
		rateOverride,
		formatOverride,
		nativeOverride,
	)
	if err != nil {
		return err
	}

	result, err := p.Profile(
		ctx,
		request,
	)
	if err != nil {
		return fmt.Errorf(
			"profile PID %d: %w",
			targetProcess.PID,
			err,
		)
	}

	printProfile(result)

	fmt.Println()

	return nil
}
