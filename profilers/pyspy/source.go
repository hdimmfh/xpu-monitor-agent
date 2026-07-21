package pyspy

import (
	"context"
	"errors"
	"fmt"
	"strings"

	coresource "github.com/hdimmfh/xpu-monitor-agent/pkg/source"
)

// EnrichSources resolves source files referenced by py-spy frames and adds
// the corresponding source path and source line text to each frame.
//
// Resolution failures are treated as non-fatal. Profiling data remains useful
// even when a source file cannot be read, for example:
//
//   - the process exited after py-spy completed
//   - the source is inside an inaccessible container filesystem
//   - the frame belongs to generated or deleted code
//   - the agent lacks filesystem permissions
//
// The returned error is reserved for invalid function arguments or context
// cancellation. Individual source resolution errors are returned separately
// in SourceEnrichmentResult.
func EnrichSources(
	ctx context.Context,
	resolver coresource.Resolver,
	pid int,
	result *DumpResult,
) (SourceEnrichmentResult, error) {
	if err := ctx.Err(); err != nil {
		return SourceEnrichmentResult{}, err
	}

	if resolver == nil {
		return SourceEnrichmentResult{}, errors.New(
			"source resolver is nil",
		)
	}

	if pid <= 0 {
		return SourceEnrichmentResult{}, fmt.Errorf(
			"PID must be greater than zero: %d",
			pid,
		)
	}

	if result == nil {
		return SourceEnrichmentResult{}, errors.New(
			"py-spy dump result is nil",
		)
	}

	enrichment := SourceEnrichmentResult{
		Errors: make(
			[]SourceEnrichmentError,
			0,
		),
	}

	cache := make(
		map[string]sourceCacheEntry,
	)

	for threadIndex := range result.Threads {
		if err := ctx.Err(); err != nil {
			return enrichment, err
		}

		thread := &result.Threads[threadIndex]

		for frameIndex := range thread.Frames {
			if err := ctx.Err(); err != nil {
				return enrichment, err
			}

			frame := &thread.Frames[frameIndex]

			file := strings.TrimSpace(
				frame.File,
			)

			if file == "" || frame.Line <= 0 {
				enrichment.SkippedFrames++
				continue
			}

			entry, exists := cache[file]
			if !exists {
				resolved, err := resolver.Resolve(
					ctx,
					coresource.ResolveRequest{
						PID:  pid,
						File: file,
					},
				)

				entry = sourceCacheEntry{
					file: resolved,
					err:  err,
				}

				cache[file] = entry
				enrichment.ResolvedFiles++

				if err != nil {
					enrichment.FailedFiles++

					enrichment.Errors = append(
						enrichment.Errors,
						SourceEnrichmentError{
							File:  file,
							Error: err.Error(),
						},
					)
				}
			}

			if entry.err != nil {
				enrichment.SkippedFrames++
				continue
			}

			lineIndex := frame.Line - 1

			if lineIndex < 0 ||
				lineIndex >= len(entry.file.Lines) {
				enrichment.SkippedFrames++

				enrichment.Errors = append(
					enrichment.Errors,
					SourceEnrichmentError{
						File: file,
						Line: frame.Line,
						Error: fmt.Sprintf(
							"source line %d is outside resolved file range 1..%d",
							frame.Line,
							len(entry.file.Lines),
						),
					},
				)

				continue
			}

			sourceLine := entry.file.Lines[lineIndex]

			frame.SourcePath = entry.file.Path
			frame.SourceText = sourceLine.Text

			enrichment.EnrichedFrames++
		}
	}

	return enrichment, nil
}

// SourceEnrichmentResult describes source enrichment activity for one py-spy
// dump result.
type SourceEnrichmentResult struct {
	ResolvedFiles  int                     `json:"resolved_files"`
	FailedFiles    int                     `json:"failed_files"`
	EnrichedFrames int                     `json:"enriched_frames"`
	SkippedFrames  int                     `json:"skipped_frames"`
	Errors         []SourceEnrichmentError `json:"errors,omitempty"`
}

// SourceEnrichmentError describes one non-fatal source enrichment failure.
type SourceEnrichmentError struct {
	File  string `json:"file"`
	Line  int    `json:"line,omitempty"`
	Error string `json:"error"`
}

type sourceCacheEntry struct {
	file coresource.SourceFile
	err  error
}
