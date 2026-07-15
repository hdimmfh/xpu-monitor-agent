package profiler

import (
	"fmt"
	"os"
	"strings"
	"time"
	"regexp"
		
	"gopkg.in/yaml.v3"
)

const (
	ModeDump   = "dump"
	ModeRecord = "record"
)

type Config struct {
	Profiling ProfilingConfig `yaml:"profiling"`
}

type ProfilingConfig struct {
	Enabled bool        `yaml:"enabled"`
	PySpy   PySpyConfig `yaml:"pyspy"`
	Storage StorageConfig `yaml:"storage"`
}

type PySpyConfig struct {
	Binary     string `yaml:"binary"`
	Mode       string `yaml:"mode"`
	Duration   string `yaml:"duration"`
	SampleRate int    `yaml:"sample_rate"`
	Format     string `yaml:"format"`
	Native     bool   `yaml:"native"`
}

type StorageConfig struct {
	Directory string `yaml:"directory"`
}

type ProfilingConfig struct {
	Enabled bool `yaml:"enabled"`

	Discovery DiscoveryConfig `yaml:"discovery"`

	PySpy PySpyConfig `yaml:"pyspy"`

	Storage StorageConfig `yaml:"storage"`
}

type DiscoveryConfig struct {
	Enabled bool `yaml:"enabled"`

	ProcRoot string `yaml:"proc_root"`

	Exclude ProcessExcludeConfig `yaml:"exclude"`
}

type ProcessExcludeConfig struct {
	PIDs []int `yaml:"pids"`

	Users []string `yaml:"users"`

	CommandRegex []string `yaml:"command_regex"`

	ExecutableRegex []string `yaml:"executable_regex"`
}

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
	if strings.TrimSpace(
		cfg.Profiling.PySpy.Binary,
	) == "" {
		cfg.Profiling.PySpy.Binary = "py-spy"
	}

	if strings.TrimSpace(
		cfg.Profiling.PySpy.Mode,
	) == "" {
		cfg.Profiling.PySpy.Mode = ModeDump
	}

	// 아래 값은 record 모드에서만 사용한다.
	if cfg.Profiling.PySpy.Duration == "" {
		cfg.Profiling.PySpy.Duration = "10s"
	}

	if cfg.Profiling.PySpy.SampleRate == 0 {
		cfg.Profiling.PySpy.SampleRate = 20
	}

	if cfg.Profiling.PySpy.Format == "" {
		cfg.Profiling.PySpy.Format = "raw"
	}

	if cfg.Profiling.Storage.Directory == "" {
		cfg.Profiling.Storage.Directory = "./profiles"
	}

	if strings.TrimSpace(
		cfg.Profiling.Discovery.ProcRoot,
	) == "" {
		cfg.Profiling.Discovery.ProcRoot = "/proc"
	}
}

func (c Config) Validate() error {
	if !c.Profiling.Enabled {
		return nil
	}

	if err := validateDiscoveryConfig(
		c.Profiling.Discovery,
	); err != nil {
		return err
	}
		
	switch c.Profiling.PySpy.Mode {
	case ModeDump:
		// dump에는 PID와 native 설정만 필요하다.
		return nil

	case ModeRecord:
		return c.validateRecordConfig()

	default:
		return fmt.Errorf(
			"unsupported profiling.pyspy.mode %q",
			c.Profiling.PySpy.Mode,
		)
	}
}

func (c Config) validateRecordConfig() error {
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
	case "raw",
		"flamegraph",
		"speedscope",
		"chrometrace":

		return nil

	default:
		return fmt.Errorf(
			"unsupported profiling.pyspy.format %q",
			c.Profiling.PySpy.Format,
		)
	}
}

func (c Config) Duration() (
	time.Duration,
	error,
) {
	return time.ParseDuration(
		c.Profiling.PySpy.Duration,
	)
}

func validateDiscoveryConfig(
	cfg DiscoveryConfig,
) error {
	if !cfg.Enabled {
		return nil
	}

	if strings.TrimSpace(cfg.ProcRoot) == "" {
		return fmt.Errorf(
			"profiling.discovery.proc_root must not be empty",
		)
	}

	for _, pattern := range cfg.Exclude.CommandRegex {
		if _, err := regexp.Compile(pattern); err != nil {
			return fmt.Errorf(
				"invalid profiling.discovery.exclude.command_regex %q: %w",
				pattern,
				err,
			)
		}
	}

	for _, pattern := range cfg.Exclude.ExecutableRegex {
		if _, err := regexp.Compile(pattern); err != nil {
			return fmt.Errorf(
				"invalid profiling.discovery.exclude.executable_regex %q: %w",
				pattern,
				err,
			)
		}
	}

	return nil
}
