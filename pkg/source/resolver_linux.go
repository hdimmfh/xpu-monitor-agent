//go:build linux

package source

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// LinuxResolver resolves source files through Linux procfs.
//
// Absolute target-process paths are resolved through:
//
//	/proc/<pid>/root/<absolute-path>
//
// Relative target-process paths are resolved through:
//
//	/proc/<pid>/cwd/<relative-path>
type LinuxResolver struct {
	procRoot    string
	maxFileSize int64
}

// LinuxResolverOption configures LinuxResolver.
type LinuxResolverOption func(
	resolver *LinuxResolver,
)

// WithProcRoot overrides the procfs root.
//
// This is primarily useful for tests. Production callers should normally
// use the default /proc root.
func WithProcRoot(
	procRoot string,
) LinuxResolverOption {
	return func(
		resolver *LinuxResolver,
	) {
		resolver.procRoot = procRoot
	}
}

// WithMaxFileSize sets the maximum source file size that may be read.
func WithMaxFileSize(
	maxFileSize int64,
) LinuxResolverOption {
	return func(
		resolver *LinuxResolver,
	) {
		resolver.maxFileSize = maxFileSize
	}
}

// NewLinuxResolver creates a Linux source resolver.
func NewLinuxResolver(
	options ...LinuxResolverOption,
) *LinuxResolver {
	resolver := &LinuxResolver{
		procRoot:    "/proc",
		maxFileSize: defaultMaxFileSize,
	}

	for _, option := range options {
		if option != nil {
			option(resolver)
		}
	}

	return resolver
}

// Resolve resolves and reads a source file from the target process filesystem.
func (r *LinuxResolver) Resolve(
	ctx context.Context,
	request ResolveRequest,
) (SourceFile, error) {
	if err := ValidateResolveRequest(request); err != nil {
		return SourceFile{}, err
	}

	if err := ctx.Err(); err != nil {
		return SourceFile{}, err
	}

	if r == nil {
		return SourceFile{}, errors.New(
			"Linux source resolver is nil",
		)
	}

	if strings.TrimSpace(r.procRoot) == "" {
		return SourceFile{}, errors.New(
			"proc root must not be empty",
		)
	}

	if r.maxFileSize <= 0 {
		return SourceFile{}, fmt.Errorf(
			"maximum source file size must be greater than zero: %d",
			r.maxFileSize,
		)
	}

	if strings.ContainsRune(
		request.File,
		'\x00',
	) {
		return SourceFile{}, errors.New(
			"source file path contains a null byte",
		)
	}

	targetPath, displayPath, err := r.resolvePath(
		request,
	)
	if err != nil {
		return SourceFile{}, err
	}

	file, err := os.Open(targetPath)
	if err != nil {
		return SourceFile{}, fmt.Errorf(
			"open source file %q for PID %d: %w",
			request.File,
			request.PID,
			err,
		)
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return SourceFile{}, fmt.Errorf(
			"stat source file %q for PID %d: %w",
			request.File,
			request.PID,
			err,
		)
	}

	if !info.Mode().IsRegular() {
		return SourceFile{}, fmt.Errorf(
			"source path %q for PID %d is not a regular file",
			request.File,
			request.PID,
		)
	}

	if info.Size() > r.maxFileSize {
		return SourceFile{}, fmt.Errorf(
			"source file %q is too large: size=%d maximum=%d",
			request.File,
			info.Size(),
			r.maxFileSize,
		)
	}

	if err := ctx.Err(); err != nil {
		return SourceFile{}, err
	}

	content, err := readLimitedFile(
		ctx,
		file,
		r.maxFileSize,
	)
	if err != nil {
		return SourceFile{}, fmt.Errorf(
			"read source file %q for PID %d: %w",
			request.File,
			request.PID,
			err,
		)
	}

	hash := sha256.Sum256(content)

	return SourceFile{
		PID:         request.PID,
		Path:        displayPath,
		ContentHash: "sha256:" + hex.EncodeToString(hash[:]),
		SizeBytes:   int64(len(content)),
		Lines:       splitSourceLines(content),
	}, nil
}

func (r *LinuxResolver) resolvePath(
	request ResolveRequest,
) (
	targetPath string,
	displayPath string,
	err error,
) {
	processRoot := filepath.Join(
		r.procRoot,
		fmt.Sprintf(
			"%d",
			request.PID,
		),
	)

	cleanedFile := filepath.Clean(
		request.File,
	)

	if filepath.IsAbs(cleanedFile) {
		relativePath := strings.TrimPrefix(
			cleanedFile,
			string(filepath.Separator),
		)

		targetPath = filepath.Join(
			processRoot,
			"root",
			relativePath,
		)

		return targetPath, cleanedFile, nil
	}

	targetPath = filepath.Join(
		processRoot,
		"cwd",
		cleanedFile,
	)

	resolvedWorkingDirectory, readLinkErr := os.Readlink(
		filepath.Join(
			processRoot,
			"cwd",
		),
	)
	if readLinkErr != nil {
		// Reading through /proc/<pid>/cwd can still work even if the
		// symlink target cannot be displayed. Keep the original relative
		// path for the API response.
		return targetPath, cleanedFile, nil
	}

	displayPath = filepath.Clean(
		filepath.Join(
			resolvedWorkingDirectory,
			cleanedFile,
		),
	)

	return targetPath, displayPath, nil
}

func readLimitedFile(
	ctx context.Context,
	reader io.Reader,
	maxFileSize int64,
) ([]byte, error) {
	limitedReader := io.LimitReader(
		reader,
		maxFileSize+1,
	)

	content, err := io.ReadAll(
		&contextReader{
			ctx:    ctx,
			reader: limitedReader,
		},
	)
	if err != nil {
		return nil, err
	}

	if int64(len(content)) > maxFileSize {
		return nil, fmt.Errorf(
			"source file exceeds maximum size of %d bytes",
			maxFileSize,
		)
	}

	return content, nil
}

func splitSourceLines(
	content []byte,
) []SourceLine {
	normalized := strings.ReplaceAll(
		string(content),
		"\r\n",
		"\n",
	)

	normalized = strings.ReplaceAll(
		normalized,
		"\r",
		"\n",
	)

	rawLines := strings.Split(
		normalized,
		"\n",
	)

	// A file ending in a newline produces an artificial final empty value
	// from strings.Split. Do not expose it as an additional source line.
	if len(rawLines) > 1 &&
		rawLines[len(rawLines)-1] == "" {
		rawLines = rawLines[:len(rawLines)-1]
	}

	lines := make(
		[]SourceLine,
		0,
		len(rawLines),
	)

	for index, text := range rawLines {
		lines = append(
			lines,
			SourceLine{
				Number: index + 1,
				Text:   text,
			},
		)
	}

	return lines
}

type contextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r *contextReader) Read(
	buffer []byte,
) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}

	count, err := r.reader.Read(buffer)
	if err != nil {
		return count, err
	}

	if contextErr := r.ctx.Err(); contextErr != nil {
		return count, contextErr
	}

	return count, nil
}
