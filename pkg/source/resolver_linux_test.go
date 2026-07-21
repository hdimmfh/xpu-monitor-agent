//go:build linux

package source

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLinuxResolverResolveAbsolutePath(t *testing.T) {
	procRoot := t.TempDir()
	pid := 1234

	processDirectory := filepath.Join(
		procRoot,
		"1234",
	)

	processRoot := filepath.Join(
		processDirectory,
		"root",
	)

	sourcePath := filepath.Join(
		processRoot,
		"workspace",
		"train.py",
	)

	if err := os.MkdirAll(
		filepath.Dir(sourcePath),
		0o755,
	); err != nil {
		t.Fatalf(
			"create source directory: %v",
			err,
		)
	}

	const content = `import torch

x = torch.ones(4, device="cuda")
y = x * 2
`

	if err := os.WriteFile(
		sourcePath,
		[]byte(content),
		0o644,
	); err != nil {
		t.Fatalf(
			"write source file: %v",
			err,
		)
	}

	resolver := NewLinuxResolver(
		WithProcRoot(procRoot),
	)

	result, err := resolver.Resolve(
		context.Background(),
		ResolveRequest{
			PID:  pid,
			File: "/workspace/train.py",
		},
	)
	if err != nil {
		t.Fatalf(
			"resolve source: %v",
			err,
		)
	}

	if result.PID != pid {
		t.Fatalf(
			"PID = %d, want %d",
			result.PID,
			pid,
		)
	}

	if result.Path != "/workspace/train.py" {
		t.Fatalf(
			"path = %q, want %q",
			result.Path,
			"/workspace/train.py",
		)
	}

	if result.SizeBytes != int64(len(content)) {
		t.Fatalf(
			"size = %d, want %d",
			result.SizeBytes,
			len(content),
		)
	}

	if len(result.Lines) != 4 {
		t.Fatalf(
			"line count = %d, want 4",
			len(result.Lines),
		)
	}

	if result.Lines[0].Number != 1 ||
		result.Lines[0].Text != "import torch" {
		t.Fatalf(
			"first line = %#v",
			result.Lines[0],
		)
	}

	if result.Lines[2].Number != 3 ||
		result.Lines[2].Text != `x = torch.ones(4, device="cuda")` {
		t.Fatalf(
			"third line = %#v",
			result.Lines[2],
		)
	}

	if !strings.HasPrefix(
		result.ContentHash,
		"sha256:",
	) {
		t.Fatalf(
			"content hash = %q",
			result.ContentHash,
		)
	}
}

func TestLinuxResolverResolveRelativePath(t *testing.T) {
	procRoot := t.TempDir()
	pid := 5678

	actualWorkingDirectory := filepath.Join(
		t.TempDir(),
		"project",
	)

	if err := os.MkdirAll(
		actualWorkingDirectory,
		0o755,
	); err != nil {
		t.Fatalf(
			"create working directory: %v",
			err,
		)
	}

	sourcePath := filepath.Join(
		actualWorkingDirectory,
		"torch_test.py",
	)

	const content = `import torch
torch.cuda.synchronize()
`

	if err := os.WriteFile(
		sourcePath,
		[]byte(content),
		0o644,
	); err != nil {
		t.Fatalf(
			"write source file: %v",
			err,
		)
	}

	processDirectory := filepath.Join(
		procRoot,
		"5678",
	)

	if err := os.MkdirAll(
		processDirectory,
		0o755,
	); err != nil {
		t.Fatalf(
			"create process directory: %v",
			err,
		)
	}

	if err := os.Symlink(
		actualWorkingDirectory,
		filepath.Join(
			processDirectory,
			"cwd",
		),
	); err != nil {
		t.Fatalf(
			"create cwd symlink: %v",
			err,
		)
	}

	resolver := NewLinuxResolver(
		WithProcRoot(procRoot),
	)

	result, err := resolver.Resolve(
		context.Background(),
		ResolveRequest{
			PID:  pid,
			File: "torch_test.py",
		},
	)
	if err != nil {
		t.Fatalf(
			"resolve source: %v",
			err,
		)
	}

	expectedPath := filepath.Join(
		actualWorkingDirectory,
		"torch_test.py",
	)

	if result.Path != expectedPath {
		t.Fatalf(
			"path = %q, want %q",
			result.Path,
			expectedPath,
		)
	}

	if len(result.Lines) != 2 {
		t.Fatalf(
			"line count = %d, want 2",
			len(result.Lines),
		)
	}

	if result.Lines[1].Text != "torch.cuda.synchronize()" {
		t.Fatalf(
			"second line = %q",
			result.Lines[1].Text,
		)
	}
}

func TestLinuxResolverRejectsInvalidPID(t *testing.T) {
	resolver := NewLinuxResolver()

	_, err := resolver.Resolve(
		context.Background(),
		ResolveRequest{
			PID:  0,
			File: "train.py",
		},
	)

	if err == nil {
		t.Fatal(
			"expected invalid PID error",
		)
	}
}

func TestLinuxResolverRejectsEmptyFile(t *testing.T) {
	resolver := NewLinuxResolver()

	_, err := resolver.Resolve(
		context.Background(),
		ResolveRequest{
			PID:  1234,
			File: "",
		},
	)

	if err == nil {
		t.Fatal(
			"expected empty source file error",
		)
	}
}

func TestLinuxResolverRejectsLargeFile(t *testing.T) {
	procRoot := t.TempDir()
	pid := 1234

	sourcePath := filepath.Join(
		procRoot,
		"1234",
		"root",
		"workspace",
		"large.py",
	)

	if err := os.MkdirAll(
		filepath.Dir(sourcePath),
		0o755,
	); err != nil {
		t.Fatalf(
			"create source directory: %v",
			err,
		)
	}

	if err := os.WriteFile(
		sourcePath,
		[]byte("0123456789"),
		0o644,
	); err != nil {
		t.Fatalf(
			"write source file: %v",
			err,
		)
	}

	resolver := NewLinuxResolver(
		WithProcRoot(procRoot),
		WithMaxFileSize(5),
	)

	_, err := resolver.Resolve(
		context.Background(),
		ResolveRequest{
			PID:  pid,
			File: "/workspace/large.py",
		},
	)

	if err == nil {
		t.Fatal(
			"expected source file size error",
		)
	}
}

func TestLinuxResolverContextCancelled(t *testing.T) {
	resolver := NewLinuxResolver()

	ctx, cancel := context.WithCancel(
		context.Background(),
	)
	cancel()

	_, err := resolver.Resolve(
		ctx,
		ResolveRequest{
			PID:  1234,
			File: "/workspace/train.py",
		},
	)

	if !errors.Is(
		err,
		context.Canceled,
	) {
		t.Fatalf(
			"error = %v, want context.Canceled",
			err,
		)
	}
}

func TestSplitSourceLinesWindowsNewlines(t *testing.T) {
	lines := splitSourceLines(
		[]byte("line one\r\nline two\r\n"),
	)

	if len(lines) != 2 {
		t.Fatalf(
			"line count = %d, want 2",
			len(lines),
		)
	}

	if lines[0].Text != "line one" {
		t.Fatalf(
			"first line = %q",
			lines[0].Text,
		)
	}

	if lines[1].Text != "line two" {
		t.Fatalf(
			"second line = %q",
			lines[1].Text,
		)
	}
}
