package source

import (
	"context"
	"errors"
	"fmt"
)

const (
	defaultMaxFileSize int64 = 4 * 1024 * 1024
)

// ResolveRequest identifies a source file visible from a target process.
//
// File may be:
//
//   - an absolute path from the target process filesystem
//   - a path relative to the target process working directory
type ResolveRequest struct {
	PID  int    `json:"pid"`
	File string `json:"file"`
}

// SourceFile is a source file resolved from a target process.
type SourceFile struct {
	PID         int          `json:"pid"`
	Path        string       `json:"path"`
	ContentHash string       `json:"content_hash"`
	SizeBytes   int64        `json:"size_bytes"`
	Lines       []SourceLine `json:"lines"`
}

// SourceLine represents one line of source code.
type SourceLine struct {
	Number int    `json:"number"`
	Text   string `json:"text"`
}

// Resolver resolves source files from target processes.
type Resolver interface {
	Resolve(
		ctx context.Context,
		request ResolveRequest,
	) (SourceFile, error)
}

// ValidateResolveRequest validates a source resolution request.
func ValidateResolveRequest(
	request ResolveRequest,
) error {
	if request.PID <= 0 {
		return fmt.Errorf(
			"PID must be greater than zero: %d",
			request.PID,
		)
	}

	if request.File == "" {
		return errors.New(
			"source file path must not be empty",
		)
	}

	return nil
}
