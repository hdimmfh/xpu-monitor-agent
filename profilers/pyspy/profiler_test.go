package pyspy

import (
	"reflect"
	"testing"
	"time"

	coreprofiler "github.com/hdimmfh/xpu-monitor-agent/pkg/profiler"
)

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

	got := buildRecordArgs(request, "/tmp/profile.txt")

	want := []string{
		"record",
		"--pid", "1234",
		"--duration", "10",
		"--rate", "20",
		"--format", "raw",
		"--output", "/tmp/profile.txt",
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

	got := buildRecordArgs(request, "/tmp/profile.txt")

	wantDuration := "2"

	if got[4] != wantDuration {
		t.Fatalf(
			"duration argument = %q, want %q",
			got[4],
			wantDuration,
		)
	}
}

func TestValidateRequestRejectsInvalidPID(t *testing.T) {
	request := coreprofiler.Request{
		Target: coreprofiler.Target{
			PID: 0,
		},
		Duration:   10 * time.Second,
		SampleRate: 20,
		Format:     "raw",
	}

	if err := validateRequest(request); err == nil {
		t.Fatal("validateRequest() error = nil, want error")
	}
}

func TestValidateRequestRejectsUnsupportedFormat(t *testing.T) {
	request := coreprofiler.Request{
		Target: coreprofiler.Target{
			PID: 1234,
		},
		Duration:   10 * time.Second,
		SampleRate: 20,
		Format:     "unknown",
	}

	if err := validateRequest(request); err == nil {
		t.Fatal("validateRequest() error = nil, want error")
	}
}
