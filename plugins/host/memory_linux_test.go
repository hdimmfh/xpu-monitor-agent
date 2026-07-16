//go:build linux

package host

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseSystemMemoryInfo(t *testing.T) {
	input := `
MemTotal:       1000000 kB
MemFree:         100000 kB
MemAvailable:    400000 kB
Buffers:          10000 kB
Cached:          200000 kB
`

	memory, err := parseSystemMemoryInfo(
		strings.NewReader(input),
	)

	require.NoError(t, err)
	require.Equal(
		t,
		uint64(1000000*1024),
		memory.TotalBytes,
	)
	require.Equal(
		t,
		uint64(100000*1024),
		memory.FreeBytes,
	)
	require.Equal(
		t,
		uint64(400000*1024),
		memory.AvailableBytes,
	)
	require.Equal(
		t,
		uint64(600000*1024),
		memory.UsedBytes,
	)
}

func TestParseSystemMemoryInfoMissingMemTotal(t *testing.T) {
	input := `
MemFree:       100000 kB
MemAvailable:  400000 kB
`

	_, err := parseSystemMemoryInfo(
		strings.NewReader(input),
	)

	require.EqualError(t, err, "MemTotal not found")
}

func TestParseSystemMemoryInfoMissingMemAvailable(t *testing.T) {
	input := `
MemTotal: 1000000 kB
MemFree:   100000 kB
`

	_, err := parseSystemMemoryInfo(
		strings.NewReader(input),
	)

	require.EqualError(t, err, "MemAvailable not found")
}

func TestParseSystemMemoryInfoMissingMemFree(t *testing.T) {
	input := `
MemTotal:     1000000 kB
MemAvailable: 400000 kB
`

	_, err := parseSystemMemoryInfo(
		strings.NewReader(input),
	)

	require.EqualError(t, err, "MemFree not found")
}

func TestParseMeminfoLine(t *testing.T) {
	key, value, ok := parseMeminfoLine(
		"MemTotal:       1024 kB",
	)

	require.True(t, ok)
	require.Equal(t, "MemTotal", key)
	require.Equal(t, uint64(1024*1024), value)
}

func TestParseMeminfoLineInvalidValue(t *testing.T) {
	_, _, ok := parseMeminfoLine(
		"MemTotal: invalid kB",
	)

	require.False(t, ok)
}
