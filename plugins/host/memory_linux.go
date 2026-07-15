//go:build linux

package host

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

const procMeminfoPath = "/proc/meminfo"

type systemMemoryInfo struct {
	TotalBytes     uint64
	FreeBytes      uint64
	AvailableBytes uint64
	UsedBytes      uint64
}

func readSystemMemoryInfo() (systemMemoryInfo, error) {
	file, err := os.Open(procMeminfoPath)
	if err != nil {
		return systemMemoryInfo{}, fmt.Errorf(
			"open %s: %w",
			procMeminfoPath,
			err,
		)
	}
	defer file.Close()

	memory, err := parseSystemMemoryInfo(file)
	if err != nil {
		return systemMemoryInfo{}, fmt.Errorf(
			"parse %s: %w",
			procMeminfoPath,
			err,
		)
	}

	return memory, nil
}

func parseSystemMemoryInfo(
	reader io.Reader,
) (systemMemoryInfo, error) {
	values := make(map[string]uint64)

	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		key, value, ok := parseMeminfoLine(scanner.Text())
		if !ok {
			continue
		}

		values[key] = value
	}

	if err := scanner.Err(); err != nil {
		return systemMemoryInfo{}, fmt.Errorf(
			"scan memory info: %w",
			err,
		)
	}

	total, ok := values["MemTotal"]
	if !ok {
		return systemMemoryInfo{}, fmt.Errorf(
			"MemTotal not found",
		)
	}

	available, ok := values["MemAvailable"]
	if !ok {
		return systemMemoryInfo{}, fmt.Errorf(
			"MemAvailable not found",
		)
	}

	free, ok := values["MemFree"]
	if !ok {
		return systemMemoryInfo{}, fmt.Errorf(
			"MemFree not found",
		)
	}

	var used uint64
	if total > available {
		used = total - available
	}

	return systemMemoryInfo{
		TotalBytes:     total,
		FreeBytes:      free,
		AvailableBytes: available,
		UsedBytes:      used,
	}, nil
}

func parseMeminfoLine(
	line string,
) (string, uint64, bool) {
	fields := strings.Fields(line)
	if len(fields) < 2 {
		return "", 0, false
	}

	key := strings.TrimSuffix(fields[0], ":")

	value, err := strconv.ParseUint(
		fields[1],
		10,
		64,
	)
	if err != nil {
		return "", 0, false
	}

	if len(fields) >= 3 && fields[2] == "kB" {
		value *= 1024
	}

	return key, value, true
}
