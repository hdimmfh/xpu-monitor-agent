package profiler

import (
	"path/filepath"
	"testing"
	"time"
)

func TestLoadRecordConfig(t *testing.T) {
	configPath := filepath.Join(
		"..",
		"..",
		"configs",
		"pyspy-record.yaml",
	)

	cfg, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	if !cfg.Profiling.Enabled {
		t.Fatal("profiling.enabled = false, want true")
	}

	if got, want := cfg.Profiling.PySpy.Mode, ModeRecord; got != want {
		t.Fatalf(
			"profiling.pyspy.mode = %q, want %q",
			got,
			want,
		)
	}

	if got, want := cfg.Profiling.PySpy.SampleRate, 20; got != want {
		t.Fatalf(
			"profiling.pyspy.sample_rate = %d, want %d",
			got,
			want,
		)
	}

	duration, err := cfg.Duration()
	if err != nil {
		t.Fatalf("Duration() error = %v", err)
	}

	if got, want := duration, 10*time.Second; got != want {
		t.Fatalf(
			"duration = %s, want %s",
			got,
			want,
		)
	}
}
