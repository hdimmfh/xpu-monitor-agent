//go:build !linux

package process

import (
	"fmt"
)

func DiscoverPython(
	procRoot string,
) ([]Process, error) {
	return nil, fmt.Errorf(
		"Python process discovery is supported only on Linux",
	)
}
