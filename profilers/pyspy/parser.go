package pyspy

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	coreprofiler "github.com/hdimmfh/xpu-monitor-agent/pkg/profiler"
)

var (
	processLinePattern = regexp.MustCompile(
		`^Process\s+(\d+):\s*(.*)$`,
	)

	pythonLinePattern = regexp.MustCompile(
		`^Python\s+(\S+)(?:\s+\((.*)\))?$`,
	)

	threadLinePattern = regexp.MustCompile(
		`^Thread\s+(\S+)(?:\s+\(([^)]*)\))?:\s*(?:"(.*)")?\s*$`,
	)
)

// DumpResult represents structured py-spy dump output.
type DumpResult struct {
	ProcessID        int          `json:"process_id,omitempty"`
	Command          string       `json:"command,omitempty"`
	PythonVersion    string       `json:"python_version,omitempty"`
	PythonExecutable string       `json:"python_executable,omitempty"`
	Threads          []DumpThread `json:"threads"`
}

// DumpThread represents one thread from py-spy dump output.
type DumpThread struct {
	ID     string      `json:"id"`
	State  string      `json:"state,omitempty"`
	Name   string      `json:"name,omitempty"`
	Frames []DumpFrame `json:"frames"`
}

// DumpFrame represents one Python or native stack frame.
type DumpFrame struct {
	Function string `json:"function"`
	File     string `json:"file,omitempty"`
	Line     int    `json:"line,omitempty"`

	// Raw contains the original frame when it could not be separated
	// into a function and source location.
	Raw string `json:"raw,omitempty"`
}

// parseProfileData converts py-spy stdout into JSON.
//
// Dump mode is parsed into DumpResult.
//
// Record mode behavior:
//
//   - speedscope and chrometrace are preserved as JSON
//   - raw and flamegraph are encoded as JSON strings
func parseProfileData(
	request coreprofiler.Request,
	output []byte,
) ([]byte, error) {
	switch request.Mode {
	case coreprofiler.ModeDump:
		parsed, err := parseDump(output)
		if err != nil {
			return nil, err
		}

		data, err := json.Marshal(parsed)
		if err != nil {
			return nil, fmt.Errorf(
				"marshal parsed py-spy dump: %w",
				err,
			)
		}

		return data, nil

	case coreprofiler.ModeRecord:
		switch request.Format {
		case "speedscope", "chrometrace":
			if !json.Valid(output) {
				return nil, fmt.Errorf(
					"py-spy %s output is not valid JSON",
					request.Format,
				)
			}

			return append(
				[]byte(nil),
				output...,
			), nil

		case "raw", "flamegraph":
			data, err := json.Marshal(
				string(output),
			)
			if err != nil {
				return nil, fmt.Errorf(
					"marshal py-spy %s output: %w",
					request.Format,
					err,
				)
			}

			return data, nil

		default:
			return nil, fmt.Errorf(
				"unsupported py-spy format %q",
				request.Format,
			)
		}

	default:
		return nil, fmt.Errorf(
			"unsupported py-spy mode %q",
			request.Mode,
		)
	}
}

// parseDump parses the text produced by:
//
//	py-spy dump --pid <pid>
func parseDump(
	output []byte,
) (DumpResult, error) {
	result := DumpResult{
		Threads: make(
			[]DumpThread,
			0,
		),
	}

	scanner := bufio.NewScanner(
		bytes.NewReader(output),
	)

	// Increase the scanner buffer in case a native frame or symbol is long.
	scanner.Buffer(
		make([]byte, 64*1024),
		1024*1024,
	)

	currentThread := -1

	for scanner.Scan() {
		rawLine := strings.TrimRight(
			scanner.Text(),
			"\r",
		)

		line := strings.TrimSpace(
			rawLine,
		)

		if line == "" {
			continue
		}

		if matches := processLinePattern.FindStringSubmatch(
			line,
		); matches != nil {
			pid, err := strconv.Atoi(
				matches[1],
			)
			if err != nil {
				return DumpResult{}, fmt.Errorf(
					"parse py-spy process ID %q: %w",
					matches[1],
					err,
				)
			}

			result.ProcessID = pid
			result.Command = strings.TrimSpace(
				matches[2],
			)

			currentThread = -1
			continue
		}

		if matches := pythonLinePattern.FindStringSubmatch(
			line,
		); matches != nil {
			result.PythonVersion = strings.TrimSpace(
				matches[1],
			)

			result.PythonExecutable = strings.TrimSpace(
				matches[2],
			)

			continue
		}

		if matches := threadLinePattern.FindStringSubmatch(
			line,
		); matches != nil {
			result.Threads = append(
				result.Threads,
				DumpThread{
					ID: strings.TrimSpace(
						matches[1],
					),
					State: strings.TrimSpace(
						matches[2],
					),
					Name: strings.TrimSpace(
						matches[3],
					),
					Frames: make(
						[]DumpFrame,
						0,
					),
				},
			)

			currentThread = len(
				result.Threads,
			) - 1

			continue
		}

		if currentThread >= 0 &&
			hasLeadingWhitespace(rawLine) {
			result.Threads[currentThread].Frames = append(
				result.Threads[currentThread].Frames,
				parseDumpFrame(line),
			)
		}
	}

	if err := scanner.Err(); err != nil {
		return DumpResult{}, fmt.Errorf(
			"scan py-spy dump output: %w",
			err,
		)
	}

	if result.ProcessID == 0 &&
		len(result.Threads) == 0 {
		return DumpResult{}, fmt.Errorf(
			"py-spy dump output did not contain a process or thread header",
		)
	}

	return result, nil
}

func parseDumpFrame(
	line string,
) DumpFrame {
	frame := DumpFrame{
		Function: line,
	}

	// Expected Python frame format:
	//
	//	synchronize (torch/cuda/__init__.py:1219)
	//
	// LastIndex is used because native symbols may contain parentheses.
	openIndex := strings.LastIndex(
		line,
		" (",
	)

	if openIndex <= 0 ||
		!strings.HasSuffix(line, ")") {
		frame.Raw = line
		return frame
	}

	frame.Function = strings.TrimSpace(
		line[:openIndex],
	)

	location := strings.TrimSpace(
		line[openIndex+2 : len(line)-1],
	)

	if location == "" {
		return frame
	}

	// Use the last colon so paths containing colons remain intact.
	colonIndex := strings.LastIndex(
		location,
		":",
	)

	if colonIndex <= 0 {
		frame.File = location
		return frame
	}

	lineNumber, err := strconv.Atoi(
		location[colonIndex+1:],
	)
	if err != nil {
		frame.File = location
		return frame
	}

	frame.File = location[:colonIndex]
	frame.Line = lineNumber

	return frame
}

func hasLeadingWhitespace(
	line string,
) bool {
	if line == "" {
		return false
	}

	return line[0] == ' ' ||
		line[0] == '\t'
}
