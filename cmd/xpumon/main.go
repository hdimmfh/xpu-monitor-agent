package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sort"
	"syscall"
	"time"

	"github.com/hdimmfh/xpu-monitor-agent/pkg/collector"
	"github.com/hdimmfh/xpu-monitor-agent/pkg/plugin"
	"github.com/hdimmfh/xpu-monitor-agent/pkg/process"
	coreprofiler "github.com/hdimmfh/xpu-monitor-agent/pkg/profiler"
	"github.com/hdimmfh/xpu-monitor-agent/plugins/host"
	"github.com/hdimmfh/xpu-monitor-agent/plugins/nvidia"
	"github.com/hdimmfh/xpu-monitor-agent/profilers/pyspy"
)

type metricsOutput struct {
	Type        string          `json:"type"`
	CollectedAt time.Time       `json:"collected_at"`
	Metrics     []plugin.Metric `json:"metrics"`
	Error       string          `json:"error,omitempty"`
}

type profileOutput struct {
	Type    string               `json:"type"`
	Profile coreprofiler.Profile `json:"profile"`
}

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

func newPlugins() ([]plugin.Plugin, func()) {
	plugins := []plugin.Plugin{
		host.New(),
	}

	closePlugins := func() {}

	nvidiaPlugin, err := nvidia.New()
	if err != nil {
		log.Printf(
			"NVIDIA plugin unavailable; continuing without NVIDIA: %v",
			err,
		)
		return plugins, closePlugins
	}

	plugins = append(plugins, nvidiaPlugin)
	closePlugins = func() {
		if err := nvidiaPlugin.Close(); err != nil {
			log.Printf("close NVIDIA plugin: %v", err)
		}
	}

	return plugins, closePlugins
}

func runCollect(ctx context.Context) error {
	plugins, closePlugins := newPlugins()
	defer closePlugins()

	c := collector.New(plugins...)
	metrics, collectErr := c.CollectAll(ctx)

	output := metricsOutput{
		Type:        "metrics",
		CollectedAt: time.Now().UTC(),
		Metrics:     metrics,
	}
	if collectErr != nil {
		output.Error = collectErr.Error()
	}

	if err := writeJSON(output); err != nil {
		return fmt.Errorf("write metric JSON: %w", err)
	}

	if collectErr != nil && len(metrics) == 0 {
		return fmt.Errorf("collect metrics: %w", collectErr)
	}

	return nil
}

func runProfile(ctx context.Context, args []string) error {
	flags := flag.NewFlagSet("profile", flag.ContinueOnError)

	configPath := flags.String(
		"config",
		"./configs/pyspy-dump.yaml",
		"path to XPUMON configuration file",
	)
	pid := flags.Int(
		"pid",
		0,
		"optional target Python process ID; omit to discover device-backed Python processes",
	)
	deviceID := flags.String(
		"device-id",
		"",
		"optional device ID filter",
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
		"emit metric JSON before profile JSON",
	)

	if err := flags.Parse(args); err != nil {
		return err
	}
	if *pid < 0 {
		return fmt.Errorf("--pid must not be negative")
	}

	cfg, err := coreprofiler.LoadConfig(*configPath)
	if err != nil {
		return err
	}
	if !cfg.Profiling.Enabled {
		return errors.New("profiling is disabled in configuration")
	}

	p, err := pyspy.New(
		pyspy.Config{
			BinaryPath: cfg.Profiling.PySpy.Binary,
		},
	)
	if err != nil {
		return fmt.Errorf("create py-spy profiler: %w", err)
	}

	plugins, closePlugins := newPlugins()
	defer closePlugins()

	if *withMetrics {
		c := collector.New(plugins...)
		metrics, collectErr := c.CollectAll(ctx)

		output := metricsOutput{
			Type:        "metrics",
			CollectedAt: time.Now().UTC(),
			Metrics:     metrics,
		}
		if collectErr != nil {
			output.Error = collectErr.Error()
		}

		if err := writeJSON(output); err != nil {
			return fmt.Errorf("write metric JSON: %w", err)
		}
	}

	devicesByPID, err := discoverDeviceProcesses(
		ctx,
		plugins,
		*deviceID,
	)
	if err != nil {
		return err
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
			devicesByPID[*pid],
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
		devicesByPID,
		*jobID,
		*durationOverride,
		*rateOverride,
		*formatOverride,
		*nativeOverride,
	)
}

func discoverDeviceProcesses(
	ctx context.Context,
	plugins []plugin.Plugin,
	requestedDeviceID string,
) (map[int][]coreprofiler.DetectedDevice, error) {
	devicesByPID := make(
		map[int][]coreprofiler.DetectedDevice,
	)

	for _, p := range plugins {
		provider, ok := p.(plugin.ProcessProvider)
		if !ok {
			continue
		}

		devices, err := p.Discover(ctx)
		if err != nil {
			log.Printf(
				"discover devices from process provider %q: %v",
				p.Name(),
				err,
			)
			continue
		}

		for _, device := range devices {
			if requestedDeviceID != "" && device.ID != requestedDeviceID {
				continue
			}

			deviceProcesses, err := provider.Processes(
				ctx,
				device.ID,
			)
			if err != nil {
				log.Printf(
					"discover processes from plugin=%q device=%q: %v",
					p.Name(),
					device.ID,
					err,
				)
				continue
			}

			for _, deviceProcess := range deviceProcesses {
				if deviceProcess.PID <= 0 {
					continue
				}

				detectedDevice := coreprofiler.DetectedDevice{
					Plugin:          p.Name(),
					ID:              device.ID,
					Vendor:          device.Vendor,
					Model:           device.Model,
					Type:            string(device.Type),
					UsedMemoryBytes: deviceProcess.UsedMemoryBytes,
					Metadata:        cloneMetadata(deviceProcess.Metadata),
				}

				devicesByPID[deviceProcess.PID] = mergeDetectedDevice(
					devicesByPID[deviceProcess.PID],
					detectedDevice,
				)
			}
		}
	}

	for pid := range devicesByPID {
		sort.Slice(
			devicesByPID[pid],
			func(i, j int) bool {
				if devicesByPID[pid][i].Plugin != devicesByPID[pid][j].Plugin {
					return devicesByPID[pid][i].Plugin < devicesByPID[pid][j].Plugin
				}
				return devicesByPID[pid][i].ID < devicesByPID[pid][j].ID
			},
		)
	}

	return devicesByPID, nil
}

func mergeDetectedDevice(
	devices []coreprofiler.DetectedDevice,
	candidate coreprofiler.DetectedDevice,
) []coreprofiler.DetectedDevice {
	for i := range devices {
		if devices[i].Plugin != candidate.Plugin ||
			devices[i].ID != candidate.ID {
			continue
		}

		if candidate.UsedMemoryBytes > devices[i].UsedMemoryBytes {
			devices[i].UsedMemoryBytes = candidate.UsedMemoryBytes
		}

		if len(candidate.Metadata) > 0 {
			if devices[i].Metadata == nil {
				devices[i].Metadata = make(map[string]string)
			}
			for key, value := range candidate.Metadata {
				devices[i].Metadata[key] = value
			}
		}

		return devices
	}

	return append(devices, candidate)
}

func cloneMetadata(
	metadata map[string]string,
) map[string]string {
	if len(metadata) == 0 {
		return nil
	}

	cloned := make(
		map[string]string,
		len(metadata),
	)
	for key, value := range metadata {
		cloned[key] = value
	}

	return cloned
}

func buildProfileRequest(
	cfg coreprofiler.Config,
	pid int,
	devices []coreprofiler.DetectedDevice,
	containerID string,
	jobID string,
	command string,
	durationOverride time.Duration,
	rateOverride int,
	formatOverride string,
	nativeOverride bool,
) (coreprofiler.Request, error) {
	hostname, err := os.Hostname()
	if err != nil {
		return coreprofiler.Request{}, fmt.Errorf(
			"get hostname: %w",
			err,
		)
	}

	request := coreprofiler.Request{
		Mode: cfg.Profiling.PySpy.Mode,
		Target: coreprofiler.Target{
			PID:         pid,
			Hostname:    hostname,
			Command:     command,
			ContainerID: containerID,
			JobID:       jobID,
			Devices:     devices,
		},
		Native: cfg.Profiling.PySpy.Native || nativeOverride,
	}

	if request.Mode == coreprofiler.ModeDump {
		return request, nil
	}

	duration, err := cfg.Duration()
	if err != nil {
		return coreprofiler.Request{}, fmt.Errorf(
			"parse profiling duration: %w",
			err,
		)
	}
	if durationOverride > 0 {
		duration = durationOverride
	}

	sampleRate := cfg.Profiling.PySpy.SampleRate
	if rateOverride > 0 {
		sampleRate = rateOverride
	}

	format := cfg.Profiling.PySpy.Format
	if formatOverride != "" {
		format = formatOverride
	}

	request.Duration = duration
	request.SampleRate = sampleRate
	request.Format = format

	return request, nil
}

func profileDiscoveredProcesses(
	ctx context.Context,
	p coreprofiler.Profiler,
	cfg coreprofiler.Config,
	devicesByPID map[int][]coreprofiler.DetectedDevice,
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
		return fmt.Errorf("discover Python processes: %w", err)
	}

	filter, err := process.NewFilter(
		cfg.Profiling.Discovery.Exclude,
	)
	if err != nil {
		return fmt.Errorf("create process filter: %w", err)
	}

	profiledCount := 0
	excludedCount := 0
	failedCount := 0
	detectedCount := 0

	for _, targetProcess := range processes {
		if err := ctx.Err(); err != nil {
			return err
		}

		devices := devicesByPID[targetProcess.PID]
		if len(devices) == 0 {
			continue
		}
		detectedCount++

		excluded, reason := filter.Excluded(targetProcess)
		if excluded {
			excludedCount++
			log.Printf(
				"skip detected Python process pid=%d command=%q reason=%s",
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
			devices,
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
				"profile detected Python process pid=%d command=%q: %v",
				targetProcess.PID,
				targetProcess.Command,
				err,
			)
			continue
		}

		profiledCount++
	}

	log.Printf(
		"device-backed Python profiling complete discovered=%d device_detected=%d profiled=%d excluded=%d failed=%d",
		len(processes),
		detectedCount,
		profiledCount,
		excludedCount,
		failedCount,
	)

	if profiledCount == 0 && failedCount > 0 {
		return fmt.Errorf(
			"all device-detected Python processes failed profiling",
		)
	}

	return nil
}

func profileSingleProcess(
	ctx context.Context,
	p coreprofiler.Profiler,
	cfg coreprofiler.Config,
	targetProcess process.Process,
	devices []coreprofiler.DetectedDevice,
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
		devices,
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

	result, err := p.Profile(ctx, request)
	if err != nil {
		return fmt.Errorf(
			"profile PID %d: %w",
			targetProcess.PID,
			err,
		)
	}

	return printProfile(result)
}

func printProfile(profile coreprofiler.Profile) error {
	return writeJSON(
		profileOutput{
			Type:    "profile",
			Profile: profile,
		},
	)
}

func writeJSON(value any) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetEscapeHTML(false)
	return encoder.Encode(value)
}

func printUsage() {
	fmt.Print(`XPUMON

Usage:
  xpumon
  xpumon collect
  xpumon profile [options]

Output:
  collect and profile commands emit JSON Lines (JSONL) to stdout.
  Logs and diagnostics are written to stderr.

Profile options:
  --config <path>
      YAML configuration file.

  --pid <pid>
      Explicit Python process ID.

      When omitted, XPUMON profiles Python processes detected by any
      registered plugin that implements plugin.ProcessProvider.

  --device-id <id>
      Optional device ID filter.

  --container-id <id>
      Container ID associated with an explicitly selected process.

  --job-id <id>
      Job ID associated with the target process.

  --command <command>
      Command associated with an explicitly selected process.

  --duration <duration>
      Override record duration. Ignored in dump mode.

  --rate <samples>
      Override record sample rate. Ignored in dump mode.

  --format <format>
      Override record output format. Ignored in dump mode.

  --native
      Include native stack frames.

  --with-metrics
      Emit metric JSON before profile JSON.
`)
}
