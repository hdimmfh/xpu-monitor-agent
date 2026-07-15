package pyspy

import (
	"encoding/json"
	"testing"

	coreprofiler "github.com/hdimmfh/xpu-monitor-agent/pkg/profiler"
)

func TestParseDump(
	t *testing.T,
) {
	const output = `Process 206450: python torch_test.py
Python v3.12.3 (/usr/bin/python3.12)

Thread 206450 (active): "MainThread"
    synchronize (torch/cuda/__init__.py:1219)
    <module> (torch_test.py:28)
`

	got, err := parseDump(
		[]byte(output),
	)
	if err != nil {
		t.Fatalf(
			"parseDump() error = %v",
			err,
		)
	}

	if got.ProcessID != 206450 {
		t.Fatalf(
			"ProcessID = %d, want 206450",
			got.ProcessID,
		)
	}

	if got.Command != "python torch_test.py" {
		t.Fatalf(
			"Command = %q, want %q",
			got.Command,
			"python torch_test.py",
		)
	}

	if got.PythonVersion != "v3.12.3" {
		t.Fatalf(
			"PythonVersion = %q, want %q",
			got.PythonVersion,
			"v3.12.3",
		)
	}

	if got.PythonExecutable != "/usr/bin/python3.12" {
		t.Fatalf(
			"PythonExecutable = %q, want %q",
			got.PythonExecutable,
			"/usr/bin/python3.12",
		)
	}

	if len(got.Threads) != 1 {
		t.Fatalf(
			"len(Threads) = %d, want 1",
			len(got.Threads),
		)
	}

	thread := got.Threads[0]

	if thread.ID != "206450" {
		t.Fatalf(
			"Thread ID = %q, want %q",
			thread.ID,
			"206450",
		)
	}

	if thread.State != "active" {
		t.Fatalf(
			"Thread State = %q, want %q",
			thread.State,
			"active",
		)
	}

	if thread.Name != "MainThread" {
		t.Fatalf(
			"Thread Name = %q, want %q",
			thread.Name,
			"MainThread",
		)
	}

	if len(thread.Frames) != 2 {
		t.Fatalf(
			"len(Frames) = %d, want 2",
			len(thread.Frames),
		)
	}

	firstFrame := thread.Frames[0]

	if firstFrame.Function != "synchronize" {
		t.Fatalf(
			"Function = %q, want %q",
			firstFrame.Function,
			"synchronize",
		)
	}

	if firstFrame.File != "torch/cuda/__init__.py" {
		t.Fatalf(
			"File = %q, want %q",
			firstFrame.File,
			"torch/cuda/__init__.py",
		)
	}

	if firstFrame.Line != 1219 {
		t.Fatalf(
			"Line = %d, want 1219",
			firstFrame.Line,
		)
	}
}

func TestParseDumpMultipleThreads(
	t *testing.T,
) {
	const output = `Process 1000: python train.py
Python v3.12.3 (/usr/bin/python3)

Thread 1000 (active): "MainThread"
    train (train.py:40)

Thread 1001 (idle): "worker"
    wait (threading.py:331)
`

	got, err := parseDump(
		[]byte(output),
	)
	if err != nil {
		t.Fatalf(
			"parseDump() error = %v",
			err,
		)
	}

	if len(got.Threads) != 2 {
		t.Fatalf(
			"len(Threads) = %d, want 2",
			len(got.Threads),
		)
	}

	if got.Threads[1].Name != "worker" {
		t.Fatalf(
			"second thread name = %q, want %q",
			got.Threads[1].Name,
			"worker",
		)
	}
}

func TestParseDumpFrame(
	t *testing.T,
) {
	tests := []struct {
		name string
		line string
		want DumpFrame
	}{
		{
			name: "Python frame",
			line: "forward (model.py:120)",
			want: DumpFrame{
				Function: "forward",
				File:     "model.py",
				Line:     120,
			},
		},
		{
			name: "Module frame",
			line: "<module> (train.py:28)",
			want: DumpFrame{
				Function: "<module>",
				File:     "train.py",
				Line:     28,
			},
		},
		{
			name: "Frame without line",
			line: "native_function (libtorch.so)",
			want: DumpFrame{
				Function: "native_function",
				File:     "libtorch.so",
			},
		},
		{
			name: "Unstructured frame",
			line: "unknown native frame",
			want: DumpFrame{
				Function: "unknown native frame",
				Raw:      "unknown native frame",
			},
		},
	}

	for _, test := range tests {
		t.Run(
			test.name,
			func(t *testing.T) {
				got := parseDumpFrame(
					test.line,
				)

				if got != test.want {
					t.Fatalf(
						"parseDumpFrame() = %#v, want %#v",
						got,
						test.want,
					)
				}
			},
		)
	}
}

func TestParseProfileDataDumpReturnsJSON(
	t *testing.T,
) {
	const output = `Process 1234: python train.py
Thread 1234 (active): "MainThread"
    train (train.py:40)
`

	data, err := parseProfileData(
		coreprofiler.Request{
			Mode: coreprofiler.ModeDump,
		},
		[]byte(output),
	)
	if err != nil {
		t.Fatalf(
			"parseProfileData() error = %v",
			err,
		)
	}

	if !json.Valid(data) {
		t.Fatalf(
			"data is not valid JSON: %s",
			data,
		)
	}

	var result DumpResult

	if err := json.Unmarshal(
		data,
		&result,
	); err != nil {
		t.Fatalf(
			"json.Unmarshal() error = %v",
			err,
		)
	}

	if result.ProcessID != 1234 {
		t.Fatalf(
			"ProcessID = %d, want 1234",
			result.ProcessID,
		)
	}
}

func TestParseProfileDataSpeedScope(
	t *testing.T,
) {
	const output = `{"$schema":"https://www.speedscope.app/file-format-schema.json"}`

	data, err := parseProfileData(
		coreprofiler.Request{
			Mode:   coreprofiler.ModeRecord,
			Format: "speedscope",
		},
		[]byte(output),
	)
	if err != nil {
		t.Fatalf(
			"parseProfileData() error = %v",
			err,
		)
	}

	if !json.Valid(data) {
		t.Fatalf(
			"data is not valid JSON: %s",
			data,
		)
	}
}

func TestParseProfileDataRejectsInvalidJSON(
	t *testing.T,
) {
	_, err := parseProfileData(
		coreprofiler.Request{
			Mode:   coreprofiler.ModeRecord,
			Format: "speedscope",
		},
		[]byte("not-json"),
	)

	if err == nil {
		t.Fatal(
			"parseProfileData() error = nil, want error",
		)
	}
}

func TestParseDumpRejectsUnknownOutput(
	t *testing.T,
) {
	_, err := parseDump(
		[]byte("unexpected output"),
	)

	if err == nil {
		t.Fatal(
			"parseDump() error = nil, want error",
		)
	}
}
