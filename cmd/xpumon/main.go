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

	durationOverride := flags.Duration(
		"duration",
		0,
		"profiling duration override",
	)

	rateOverride := flags.Int(
		"rate",
		0,
		"sampling rate override",
	)

	formatOverride := flags.String(
		"format",
		"",
		"output format override",
	)

	nativeOverride := flags.Bool(
		"native",
		false,
		"collect native stack frames",
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

	p, err := pyspy.New(pyspy.Config{
		BinaryPath: cfg.Profiling.PySpy.Binary,
		OutputDir:  cfg.Profiling.Storage.Directory,
	})
	if err != nil {
		return fmt.Errorf("create py-spy profiler: %w", err)
	}

	request := coreprofiler.Request{
		Target: coreprofiler.Target{
			PID: *pid,
		},
		Duration:   duration,
		SampleRate: sampleRate,
		Format:     format,
		Native:     native,
	}

	result, err := p.Profile(ctx, request)
	if err != nil {
		return fmt.Errorf("profile PID %d: %w", *pid, err)
	}

	fmt.Printf("profiler=%s\n", result.Profiler)
	fmt.Printf("pid=%d\n", result.Target.PID)
	fmt.Printf("started_at=%s\n", result.StartedAt.Format(time.RFC3339Nano))
	fmt.Printf("ended_at=%s\n", result.EndedAt.Format(time.RFC3339Nano))
	fmt.Printf("format=%s\n", result.Format)
	fmt.Printf("output=%s\n", result.OutputPath)
	fmt.Printf("metadata=%s\n", result.MetadataPath)

	return nil
}

func printUsage() {
	fmt.Println(`XPUMON

Usage:
  xpumon collect
  xpumon profile --pid <PID> [options]

Profile options:
  --config <path>      configuration file
  --pid <PID>          target Python process ID
  --duration <value>   profiling duration override
  --rate <number>      sampling rate override
  --format <format>    raw, flamegraph, speedscope, chrometrace
  --native             include native stack frames

Examples:
  xpumon collect
  xpumon profile --pid 1234
  xpumon profile --pid 1234 --duration 30s
  xpumon profile --pid 1234 --format speedscope
`)
}
