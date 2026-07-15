package process

import (
	"fmt"
	"regexp"
	"slices"

	coreprofiler "github.com/hdimmfh/xpu-monitor-agent/pkg/profiler"
)

type Filter struct {
	excludedPIDs []int
	excludedUsers []string

	commandPatterns []*regexp.Regexp

	executablePatterns []*regexp.Regexp
}

func NewFilter(
	cfg coreprofiler.ProcessExcludeConfig,
) (
	*Filter,
	error,
) {
	commandPatterns, err := compilePatterns(
		cfg.CommandRegex,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"compile command exclusion patterns: %w",
			err,
		)
	}

	executablePatterns, err := compilePatterns(
		cfg.ExecutableRegex,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"compile executable exclusion patterns: %w",
			err,
		)
	}

	return &Filter{
		excludedPIDs: cfg.PIDs,

		excludedUsers: cfg.Users,

		commandPatterns: commandPatterns,

		executablePatterns: executablePatterns,
	}, nil
}

func (f *Filter) Excluded(
	process Process,
) (
	bool,
	string,
) {
	if slices.Contains(
		f.excludedPIDs,
		process.PID,
	) {
		return true, "pid"
	}

	if process.User != "" &&
		slices.Contains(
			f.excludedUsers,
			process.User,
		) {
		return true, "user"
	}

	for _, pattern := range f.commandPatterns {
		if pattern.MatchString(
			process.Command,
		) {
			return true, "command_regex"
		}
	}

	for _, pattern := range f.executablePatterns {
		if pattern.MatchString(
			process.Executable,
		) {
			return true, "executable_regex"
		}
	}

	return false, ""
}

func compilePatterns(
	patterns []string,
) (
	[]*regexp.Regexp,
	error,
) {
	compiled := make(
		[]*regexp.Regexp,
		0,
		len(patterns),
	)

	for _, pattern := range patterns {
		expression, err := regexp.Compile(pattern)
		if err != nil {
			return nil, fmt.Errorf(
				"compile pattern %q: %w",
				pattern,
				err,
			)
		}

		compiled = append(
			compiled,
			expression,
		)
	}

	return compiled, nil
}
