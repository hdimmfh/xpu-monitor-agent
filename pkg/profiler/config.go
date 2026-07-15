package profiler

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Config is the root XPUMON profiler configuration.
type Config struct {
	Profiling ProfilingConfig `yaml:"profiling"`
}

// ProfilingConfig controls whether profiling is enabled and configures
// the concrete profiler implementation.
type ProfilingConfig struct {
	Enabled bool        `yaml:"enabled"`
	PySpy   PySpyConfig `yaml:"pyspy"`
}

// PySpyConfig defines the default py-spy execution options.
//
// These values may later be overridden by CLI flags or API request
// parameters.
type PySpyConfig struct {
	Binary     string `yaml:"binary"`
	Duration   string `yaml:"duration"`
	SampleRate int    `yaml:"sample_rate"`
	Format     string `yaml:"format"`
	Native     bool   `yaml:"native"`
}

// LoadConfig reads, applies defaults to, and validates a profiler
// configuration file.
func LoadConfig(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf(
			"read profiler config %q: %w",
			path,
			err,
		)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf(
			"unmarshal profiler config %q: %w",
			path,
			err,
		)
	}

	applyDefaults(&cfg)

	if err := cfg.Validate(); err != nil {
		return Config{}, fmt.Errorf(
			"validate profiler config: %w",
			err,
		)
	}

	return cfg, nil
}

func applyDefaults(cfg *Config) {
	if cfg.Profiling.PySpy.Binary == "" {
		cfg.Profiling.PySpy.Binary = "py-spy"
	}

	if cfg.Profiling.PySpy.Duration == "" {
		cfg.Profiling.PySpy.Duration = "10s"
	}

	if cfg.Profiling.PySpy.SampleRate == 0 {
		cfg.Profiling.PySpy.SampleRate = 20
	}

	if cfg.Profiling.PySpy.Format == "" {
		cfg.Profiling.PySpy.Format = "raw"
	}
}

// Validate checks enabled profiler settings.
func (c Config) Validate() error {
	if !c.Profiling.Enabled {
		return nil
	}

	duration, err := time.ParseDuration(
		c.Profiling.PySpy.Duration,
	)
	if err != nil {
		return fmt.Errorf(
			"invalid profiling.pyspy.duration %q: %w",
			c.Profiling.PySpy.Duration,
			err,
		)
	}

	if duration <= 0 {
		return fmt.Errorf(
			"profiling.pyspy.duration must be greater than zero",
		)
	}

	if c.Profiling.PySpy.SampleRate <= 0 {
		return fmt.Errorf(
			"profiling.pyspy.sample_rate must be greater than zero",
		)
	}

	switch c.Profiling.PySpy.Format {
	case "raw", "flamegraph", "speedscope", "chrometrace":
	default:
		return fmt.Errorf(
			"unsupported profiling.pyspy.format %q",
			c.Profiling.PySpy.Format,
		)
	}

	return nil
}

// Duration returns the configured py-spy duration.
func (c Config) Duration() (time.Duration, error) {
	return time.ParseDuration(c.Profiling.PySpy.Duration)
}
