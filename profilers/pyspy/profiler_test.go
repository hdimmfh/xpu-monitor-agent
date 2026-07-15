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

func TestNewUsesDefaultBinaryPath(
	t *testing.T,
) {
	p, err := New(Config{})
	if err != nil {
		t.Fatalf(
			"New() error = %v",
			err,
		)
	}

	if p.binaryPath != "py-spy" {
		t.Fatalf(
			"binaryPath = %q, want %q",
			p.binaryPath,
			"py-spy",
		)
	}
}

func TestName(
	t *testing.T,
) {
	p, err := New(Config{})
	if err != nil {
		t.Fatalf(
			"New() error = %v",
			err,
		)
	}

	if got, want :=
		p.Name(),
		"py-spy";
		got != want {

		t.Fatalf(
			"Name() = %q, want %q",
			got,
			want,
		)
	}
}

func TestBuildDumpArgs(
	t *testing.T,
) {
	request := coreprofiler.Request{
		Mode: coreprofiler.ModeDump,

		Target: coreprofiler.Target{
			PID: 1234,
		},

		Native: true,
	}

	got := buildDumpArgs(request)

	want := []string{
		"dump",
		"--pid",
		"1234",
		"--native",
	}

	if !reflect.DeepEqual(
		got,
		want,
	) {
		t.Fatalf(
			"buildDumpArgs() = %#v, want %#v",
			got,
			want,
		)
	}
}

func TestBuildRecordArgs(
	t *testing.T,
) {
	request := validRecordRequest()

	request.Native = true

	got := buildRecordArgs(
		request,
	)

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

	if !reflect.DeepEqual(
		got,
		want,
	) {
		t.Fatalf(
			"buildRecordArgs() = %#v, want %#v",
			got,
			want,
		)
	}
}

func TestValidateDumpRequest(
	t *testing.T,
) {
	request := validDumpRequest()

	if err := validateRequest(
		request,
	); err != nil {
		t.Fatalf(
			"validateRequest() error = %v",
			err,
		)
	}
}

func TestDumpDoesNotRequireRecordFields(
	t *testing.T,
) {
	request := validDumpRequest()

	request.Duration = 0
	request.SampleRate = 0
	request.Format = ""

	if err := validateRequest(
		request,
	); err != nil {
		t.Fatalf(
			"validateRequest() error = %v",
			err,
		)
	}
}

func TestValidateRecordRequest(
	t *testing.T,
) {
	request := validRecordRequest()

	if err := validateRequest(
		request,
	); err != nil {
		t.Fatalf(
			"validateRequest() error = %v",
			err,
		)
	}
}

func TestValidateRejectsUnknownMode(
	t *testing.T,
) {
	request := validDumpRequest()

	request.Mode = "unknown"

	err := validateRequest(request)

	if err == nil {
		t.Fatal(
			"validateRequest() error = nil",
		)
	}

	if !strings.Contains(
		err.Error(),
		"unsupported py-spy mode",
	) {
		t.Fatalf(
			"unexpected error: %v",
			err,
		)
	}
}

func TestProfileDump(
	t *testing.T,
) {
	if runtime.GOOS == "windows" {
		t.Skip(
			"test uses shell script",
		)
	}

	const dumpData = `Process 1234: python train.py

Thread 1234 (active): "MainThread"
    forward (train.py:20)
    train (train.py:40)
`

	binaryPath := writeFakePySpy(
		t,
		`
if [ "$1" = "--version" ]; then
	printf '%s\n' 'py-spy 0.4.1'
	exit 0
fi

if [ "$1" = "dump" ]; then
	cat <<'PROFILE'
Process 1234: python train.py

Thread 1234 (active): "MainThread"
    forward (train.py:20)
    train (train.py:40)
PROFILE
	exit 0
fi

exit 1
`,
	)

	p, err := New(
		Config{
			BinaryPath: binaryPath,
		},
	)
	if err != nil {
		t.Fatalf(
			"New() error = %v",
			err,
		)
	}

	result, err := p.Profile(
		context.Background(),
		validDumpRequest(),
	)
	if err != nil {
		t.Fatalf(
			"Profile() error = %v",
			err,
		)
	}

	if result.Mode !=
		coreprofiler.ModeDump {

		t.Fatalf(
			"Mode = %q",
			result.Mode,
		)
	}

	if result.Format != "text" {
		t.Fatalf(
			"Format = %q",
			result.Format,
		)
	}

	if result.Text() != dumpData {
		t.Fatalf(
			"Text() = %q, want %q",
			result.Text(),
			dumpData,
		)
	}
}

func TestProfileRecord(
	t *testing.T,
) {
	if runtime.GOOS == "windows" {
		t.Skip(
			"test uses shell script",
		)
	}

	const recordData = `train;forward 20
train;backward 10
`

	binaryPath := writeFakePySpy(
		t,
		`
if [ "$1" = "--version" ]; then
	printf '%s\n' 'py-spy 0.4.1'
	exit 0
fi

if [ "$1" = "record" ]; then
	cat <<'PROFILE'
train;forward 20
train;backward 10
PROFILE
	exit 0
fi

exit 1
`,
	)

	p, err := New(
		Config{
			BinaryPath: binaryPath,
		},
	)
	if err != nil {
		t.Fatalf(
			"New() error = %v",
			err,
		)
	}

	result, err := p.Profile(
		context.Background(),
		validRecordRequest(),
	)
	if err != nil {
		t.Fatalf(
			"Profile() error = %v",
			err,
		)
	}

	if result.Mode !=
		coreprofiler.ModeRecord {

		t.Fatalf(
			"Mode = %q",
			result.Mode,
		)
	}

	if result.Format != "raw" {
		t.Fatalf(
			"Format = %q",
			result.Format,
		)
	}

	if result.Text() != recordData {
		t.Fatalf(
			"Text() = %q, want %q",
			result.Text(),
			recordData,
		)
	}
}

func TestProfileRejectsInvalidPID(
	t *testing.T,
) {
	p, err := New(Config{})
	if err != nil {
		t.Fatalf(
			"New() error = %v",
			err,
		)
	}

	request := validDumpRequest()

	request.Target.PID = 0

	result, err := p.Profile(
		context.Background(),
		request,
	)

	if err == nil {
		t.Fatal(
			"Profile() error = nil",
		)
	}

	if result.Error == "" {
		t.Fatal(
			"result.Error is empty",
		)
	}
}

func TestProfileHonorsCanceledContext(
	t *testing.T,
) {
	if runtime.GOOS == "windows" {
		t.Skip(
			"test uses shell script",
		)
	}

	binaryPath := writeFakePySpy(
		t,
		`
if [ "$1" = "--version" ]; then
	printf '%s\n' 'py-spy 0.4.1'
	exit 0
fi

exit 1
`,
	)

	p, err := New(
		Config{
			BinaryPath: binaryPath,
		},
	)
	if err != nil {
		t.Fatalf(
			"New() error = %v",
			err,
		)
	}

	ctx, cancel := context.WithCancel(
		context.Background(),
	)

	cancel()

	result, err := p.Profile(
		ctx,
		validDumpRequest(),
	)

	if err == nil {
		t.Fatal(
			"Profile() error = nil",
		)
	}

	if !errors.Is(
		err,
		context.Canceled,
	) {
		t.Fatalf(
			"error = %v",
			err,
		)
	}

	if result.Error == "" {
		t.Fatal(
			"result.Error is empty",
		)
	}
}

func validDumpRequest() (
	coreprofiler.Request
) {
	return coreprofiler.Request{
		Mode: coreprofiler.ModeDump,

		Target: coreprofiler.Target{
			PID: 1234,
		},
	}
}

func validRecordRequest() (
	coreprofiler.Request
) {
	return coreprofiler.Request{
		Mode: coreprofiler.ModeRecord,

		Target: coreprofiler.Target{
			PID: 1234,
		},

		Duration:
			10 * time.Second,

		SampleRate: 20,

		Format: "raw",
	}
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

	content :=
		"#!/bin/sh\nset -eu\n" +
			body

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
