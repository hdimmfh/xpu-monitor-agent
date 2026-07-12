package main

import (
	"context"
	"fmt"
	"log"

	"github.com/hdimmfh/xpu-monitor-agent/pkg/collector"
	"github.com/hdimmfh/xpu-monitor-agent/plugins/nvidia"
)

func main() {
	nvidiaPlugin, err := nvidia.New()
	if err != nil {
		log.Fatalf(
			"create NVIDIA plugin: %v",
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

	c := collector.New(nvidiaPlugin)

	metrics, err := c.CollectAll(
		context.Background(),
	)
	if err != nil {
		log.Fatalf(
			"collect metrics: %v",
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
}
