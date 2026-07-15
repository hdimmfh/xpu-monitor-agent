package pyspy

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	coreprofiler "github.com/hdimmfh/xpu-monitor-agent/pkg/profiler"
)

func TestNewUsesDefaultBinaryPath(t *testing.T) {
	p, err := New(Config{})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if p.binaryPath != "py-spy" {
		t.Fatalf(
			"binaryPath = %q, want %q",
			p.binaryPath,
			"py-spy",
		)
	}
}

func TestNewUsesConfiguredBinaryPath(t *testing.T) {
	p, err := New(Config{
		BinaryPath: "/usr/local/bin/py-spy",
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if p.binaryPath != "/usr/local/bin/py-spy" {
		t.Fatalf(
			"binaryPath = %q, want %q",
			p.binaryPath,
			"/usr/local/bin/py-spy",
		)
	}
}

func TestName(t *testing.T) {
	p, err := New(Config{})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if got, want := p.Name(), "py-spy"; got != want {
		t.Fatalf(
			"Name() = %q, want %q",
			got,
			want,
		)
	}
}

func TestBuildRecordArgs(t *testing.T) {
	request := coreprofiler.Request{
		Target: coreprofiler.Target{
			PID: 1234,
		},
		Duration:   10 * time.Second,
		SampleRate: 20,
		Format:     "raw",
		Native:     true,
	}

	got := buildRecordArgs(request)

	want := []string{
		"record",
		"--pid",
		"1234",
		"--duration",
		"10",
		"--rate",
		"20",
		"--format",
		"raw",
		"--output",
		"/dev/stdout",
		"--native",
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf(
			"buildRecordArgs() = %#v, want %#v",
			got,
			want,
		)
	}
}

func TestBuildRecordArgsRoundsDurationUp(t *testing.T) {
	request := coreprofiler.Request{
		Target: coreprofiler.Target{
			PID: 1234,
		},
		Duration:   1500 * time.Millisecond,
		SampleRate: 20,
		Format:     "raw",
	}

	got := buildRecordArgs(request)

	wantDuration := "2"

	durationIndex := indexOf(got, "--duration")
	if durationIndex == -1 {
		t.Fatal(`buildRecordArgs() does not contain "--duration"`)
	}

	valueIndex := durationIndex + 1
	if valueIndex >= len(got) {
		t.Fatal(`buildRecordArgs() has no value after "--duration"`)
	}

	if got[valueIndex] != wantDuration {
		t.Fatalf(
			"duration argument = %q, want %q",
			got[valueIndex],
			wantDuration,
		)
	}
}

func TestBuildRecordArgsDoesNotAddNativeByDefault(t *testing.T) {
	request := validRequest()
	request.Native = false

	got := buildRecordArgs(request)

	if indexOf(got, "--native") != -1 {
		t.Fatalf(
			"buildRecordArgs() = %#v, must not contain --native",
			got,
		)
	}
}

func TestValidateRequest(t *testing.T) {
	tests := []struct {
		name        string
		request     coreprofiler.Request
		wantErrText string
	}{
		{
			name:        "invalid PID",
			request:     requestWithPID(0),
			wantErrText: "PID must be greater than zero",
		},
		{
			name: "invalid duration",
			request: func() coreprofiler.Request {
				request := validRequest()
				request.Duration = 0
				return request
			}(),
			wantErrText: "duration must be greater than zero",
		},
		{
			name: "invalid sample rate",
			request: func() coreprofiler.Request {
				request := validRequest()
				request.SampleRate = 0
				return request
			}(),
			wantErrText: "sample rate must be greater than zero",
		},
		{
			name: "unsupported format",
			request: func() coreprofiler.Request {
				request := validRequest()
				request.Format = "unknown"
				return request
			}(),
			wantErrText: `unsupported py-spy format "unknown"`,
		},
		{
			name:    "raw format",
			request: validRequest(),
		},
		{
			name: "flamegraph format",
			request: func() coreprofiler.Request {
				request := validRequest()
				request.Format = "flamegraph"
				return request
			}(),
		},
		{
			name: "speedscope format",
			request: func() coreprofiler.Request {
				request := validRequest()
				request.Format = "speedscope"
				return request
			}(),
		},
		{
			name: "chrometrace format",
			request: func() coreprofiler.Request {
				request := validRequest()
				request.Format = "chrometrace"
				return request
			}(),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validateRequest(test.request)

			if test.wantErrText == "" {
				if err != nil {
					t.Fatalf(
						"validateRequest() error = %v, want nil",
						err,
					)
				}

				return
			}

			if err == nil {
				t.Fatalf(
					"validateRequest() error = nil, want %q",
					test.wantErrText,
				)
			}

			if !strings.Contains(err.Error(), test.wantErrText) {
				t.Fatalf(
					"validateRequest() error = %q, want containing %q",
					err,
					test.wantErrText,
				)
			}
		})
	}
}

func TestAvailable(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test uses a Unix shell script")
	}

	binaryPath := writeFakePySpy(t, `
if [ "$1" = "--version" ]; then
	printf '%s\n' 'py-spy 0.4.1'
	exit 0
fi

exit 1
`)

	p, err := New(Config{
		BinaryPath: binaryPath,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if err := p.Available(context.Background()); err != nil {
		t.Fatalf("Available() error = %v", err)
	}
}

func TestAvailableReturnsErrorWhenBinaryDoesNotExist(t *testing.T) {
	p, err := New(Config{
		BinaryPath: filepath.Join(
			t.TempDir(),
			"missing-py-spy",
		),
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	err = p.Available(context.Background())
	if err == nil {
		t.Fatal("Available() error = nil, want error")
	}

	if !strings.Contains(err.Error(), "find py-spy binary") {
		t.Fatalf(
			"Available() error = %q, want binary lookup error",
			err,
		)
	}
}

func TestProfileReturnsOutputInMemory(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test uses a Unix shell script")
	}

	const profileData = `<module> (torch_test.py:22);synchronize (torch/cuda/__init__.py:1219) 50
<module> (torch_test.py:24);synchronize (torch/cuda/__init__.py:1219) 44
`

	binaryPath := writeFakePySpy(t, `
if [ "$1" = "--version" ]; then
	printf '%s\n' 'py-spy 0.4.1'
	exit 0
fi

if [ "$1" = "record" ]; then
	cat <<'PROFILE'
<module> (torch_test.py:22);synchronize (torch/cuda/__init__.py:1219) 50
<module> (torch_test.py:24);synchronize (torch/cuda/__init__.py:1219) 44
PROFILE
	printf '%s\n' 'fake progress message' >&2
	exit 0
fi

exit 1
`)

	p, err := New(Config{
		BinaryPath: binaryPath,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	request := coreprofiler.Request{
		Target: coreprofiler.Target{
			PID:         124243,
			DeviceID:    "GPU-test",
			Hostname:    "node-test",
			Command:     "python torch_test.py",
			ContainerID: "container-test",
			JobID:       "job-test",
		},
		Duration:   10 * time.Second,
		SampleRate: 20,
		Format:     "raw",
	}

	result, err := p.Profile(
		context.Background(),
		request,
	)
	if err != nil {
		t.Fatalf("Profile() error = %v", err)
	}

	if result.Profiler != "py-spy" {
		t.Fatalf(
			"Profiler = %q, want %q",
			result.Profiler,
			"py-spy",
		)
	}

	if !reflect.DeepEqual(result.Target, request.Target) {
		t.Fatalf(
			"Target = %#v, want %#v",
			result.Target,
			request.Target,
		)
	}

	if result.Format != "raw" {
		t.Fatalf(
			"Format = %q, want %q",
			result.Format,
			"raw",
		)
	}

	if result.Text() != profileData {
		t.Fatalf(
			"Text() = %q, want %q",
			result.Text(),
			profileData,
		)
	}

	if strings.Contains(
		result.Text(),
		"fake progress message",
	) {
		t.Fatal(
			"Profile data contains stderr diagnostic output",
		)
	}

	if result.StartedAt.IsZero() {
		t.Fatal("StartedAt is zero")
	}

	if result.EndedAt.IsZero() {
		t.Fatal("EndedAt is zero")
	}

	if result.EndedAt.Before(result.StartedAt) {
		t.Fatalf(
			"EndedAt %s is before StartedAt %s",
			result.EndedAt,
			result.StartedAt,
		)
	}

	if result.Error != "" {
		t.Fatalf(
			"Error = %q, want empty",
			result.Error,
		)
	}
}

func TestProfileReturnsPySpyError(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test uses a Unix shell script")
	}

	binaryPath := writeFakePySpy(t, `
if [ "$1" = "--version" ]; then
	printf '%s\n' 'py-spy 0.4.1'
	exit 0
fi

if [ "$1" = "record" ]; then
	printf '%s\n' 'permission denied while attaching to process' >&2
	exit 1
fi

exit 1
`)

	p, err := New(Config{
		BinaryPath: binaryPath,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	result, err := p.Profile(
		context.Background(),
		validRequest(),
	)
	if err == nil {
		t.Fatal("Profile() error = nil, want error")
	}

	if !strings.Contains(
		err.Error(),
		"permission denied while attaching to process",
	) {
		t.Fatalf(
			"Profile() error = %q, want stderr message",
			err,
		)
	}

	if result.Error == "" {
		t.Fatal("result.Error is empty, want error message")
	}
}

func TestProfileRejectsInvalidRequestBeforeExecution(t *testing.T) {
	p, err := New(Config{
		BinaryPath: filepath.Join(
			t.TempDir(),
			"missing-py-spy",
		),
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	request := validRequest()
	request.Target.PID = 0

	result, err := p.Profile(
		context.Background(),
		request,
	)
	if err == nil {
		t.Fatal("Profile() error = nil, want validation error")
	}

	if !strings.Contains(
		err.Error(),
		"PID must be greater than zero",
	) {
		t.Fatalf(
			"Profile() error = %q, want PID validation error",
			err,
		)
	}

	if result.Error == "" {
		t.Fatal("result.Error is empty, want validation error")
	}
}

func TestProfileHonorsCanceledContext(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test uses a Unix shell script")
	}

	binaryPath := writeFakePySpy(t, `
if [ "$1" = "--version" ]; then
	printf '%s\n' 'py-spy 0.4.1'
	exit 0
fi

exit 1
`)

	p, err := New(Config{
		BinaryPath: binaryPath,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	ctx, cancel := context.WithCancel(
		context.Background(),
	)
	cancel()

	result, err := p.Profile(ctx, validRequest())
	if err == nil {
		t.Fatal("Profile() error = nil, want context error")
	}

	if !errors.Is(err, context.Canceled) {
		t.Fatalf(
			"Profile() error = %v, want context.Canceled",
			err,
		)
	}

	if result.Error == "" {
		t.Fatal("result.Error is empty, want context error")
	}
}

func validRequest() coreprofiler.Request {
	return coreprofiler.Request{
		Target: coreprofiler.Target{
			PID: 1234,
		},
		Duration:   10 * time.Second,
		SampleRate: 20,
		Format:     "raw",
	}
}

func requestWithPID(pid int) coreprofiler.Request {
	request := validRequest()
	request.Target.PID = pid

	return request
}

func indexOf(values []string, target string) int {
	for index, value := range values {
		if value == target {
			return index
		}
	}

	return -1
}

func writeFakePySpy(
	t *testing.T,
	body string,
) string {
	t.Helper()

	path := filepath.Join(
		t.TempDir(),
		"py-spy",
	)

	content := "#!/bin/sh\nset -eu\n" + body

	if err := os.WriteFile(
		path,
		[]byte(content),
		0o700,
	); err != nil {
		t.Fatalf(
			"write fake py-spy: %v",
			err,
		)
	}

	return path
}
