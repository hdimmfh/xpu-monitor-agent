//go:build linux

package process

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
)

func DiscoverPython(
	procRoot string,
) ([]Process, error) {
	entries, err := os.ReadDir(procRoot)
	if err != nil {
		return nil, fmt.Errorf(
			"read proc root %q: %w",
			procRoot,
			err,
		)
	}

	processes := make(
		[]Process,
		0,
	)

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		pid, err := strconv.Atoi(entry.Name())
		if err != nil || pid <= 0 {
			continue
		}

		process, err := readProcess(
			procRoot,
			pid,
		)
		if err != nil {
			// 프로세스가 탐색 도중 종료되거나 권한이 없는 경우는
			// 전체 discovery 실패로 처리하지 않는다.
			if errors.Is(err, fs.ErrNotExist) ||
				errors.Is(err, fs.ErrPermission) {
				continue
			}

			continue
		}

		if !isPythonExecutable(
			process.Executable,
		) {
			continue
		}

		processes = append(
			processes,
			process,
		)
	}

	return processes, nil
}

func readProcess(
	procRoot string,
	pid int,
) (
	Process,
	error,
) {
	pidRoot := filepath.Join(
		procRoot,
		strconv.Itoa(pid),
	)

	executable, err := os.Readlink(
		filepath.Join(
			pidRoot,
			"exe",
		),
	)
	if err != nil {
		return Process{}, fmt.Errorf(
			"read executable for PID %d: %w",
			pid,
			err,
		)
	}

	command, err := readCommand(
		filepath.Join(
			pidRoot,
			"cmdline",
		),
	)
	if err != nil {
		return Process{}, fmt.Errorf(
			"read command for PID %d: %w",
			pid,
			err,
		)
	}

	username, err := readUsername(
		filepath.Join(
			pidRoot,
			"status",
		),
	)
	if err != nil {
		// 사용자 조회 실패는 profile 대상 판별에 필수적이지 않다.
		username = ""
	}

	return Process{
		PID:        pid,
		Executable: executable,
		Command:    command,
		User:       username,
	}, nil
}

func readCommand(
	path string,
) (
	string,
	error,
) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}

	fields := bytes.FieldsFunc(
		data,
		func(r rune) bool {
			return r == '\x00'
		},
	)

	arguments := make(
		[]string,
		0,
		len(fields),
	)

	for _, field := range fields {
		arguments = append(
			arguments,
			string(field),
		)
	}

	return strings.Join(
		arguments,
		" ",
	), nil
}

func readUsername(
	statusPath string,
) (
	string,
	error,
) {
	data, err := os.ReadFile(statusPath)
	if err != nil {
		return "", err
	}

	for _, line := range strings.Split(
		string(data),
		"\n",
	) {
		if !strings.HasPrefix(
			line,
			"Uid:",
		) {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 2 {
			return "", fmt.Errorf(
				"invalid Uid line %q",
				line,
			)
		}

		account, err := user.LookupId(
			fields[1],
		)
		if err != nil {
			// 사용자 DB 조회가 안 되면 UID 문자열을 그대로 사용한다.
			return fields[1], nil
		}

		return account.Username, nil
	}

	return "", fmt.Errorf(
		"Uid field not found in %q",
		statusPath,
	)
}

func isPythonExecutable(
	executablePath string,
) bool {
	name := strings.ToLower(
		filepath.Base(executablePath),
	)

	if name == "python" ||
		name == "python2" ||
		name == "python3" {
		return true
	}

	return strings.HasPrefix(
		name,
		"python2.",
	) || strings.HasPrefix(
		name,
		"python3.",
	)
}
