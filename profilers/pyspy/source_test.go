package pyspy

import (
	"context"
	"errors"
	"testing"

	coresource "github.com/hdimmfh/xpu-monitor-agent/pkg/source"
)

type fakeSourceResolver struct {
	results map[string]coresource.SourceFile
	errors  map[string]error
	calls   map[string]int
}

func (f *fakeSourceResolver) Resolve(
	ctx context.Context,
	request coresource.ResolveRequest,
) (coresource.SourceFile, error) {
	if err := ctx.Err(); err != nil {
		return coresource.SourceFile{}, err
	}

	if f.calls == nil {
		f.calls = make(
			map[string]int,
		)
	}

	f.calls[request.File]++

	if err, exists := f.errors[request.File]; exists {
		return coresource.SourceFile{}, err
	}

	result, exists := f.results[request.File]
	if !exists {
		return coresource.SourceFile{}, errors.New(
			"source not found",
		)
	}

	return result, nil
}

func TestEnrichSources(t *testing.T) {
	resolver := &fakeSourceResolver{
		results: map[string]coresource.SourceFile{
			"torch_test.py": {
				PID:  31736,
				Path: "/root/torch_test.py",
				Lines: []coresource.SourceLine{
					{
						Number: 1,
						Text:   "import torch",
					},
					{
						Number: 2,
						Text:   "",
					},
					{
						Number: 3,
						Text:   "x = torch.ones(4, device=\"cuda\")",
					},
					{
						Number: 4,
						Text:   "torch.cuda.synchronize()",
					},
				},
			},
		},
	}

	result := DumpResult{
		ProcessID: 31736,
		Threads: []DumpThread{
			{
				ID:    "31736",
				State: "active",
				Name:  "MainThread",
				Frames: []DumpFrame{
					{
						Function: "synchronize",
						File:     "torch_test.py",
						Line:     4,
					},
					{
						Function: "<module>",
						File:     "torch_test.py",
						Line:     3,
					},
				},
			},
		},
	}

	enrichment, err := EnrichSources(
		context.Background(),
		resolver,
		31736,
		&result,
	)
	if err != nil {
		t.Fatalf(
			"enrich sources: %v",
			err,
		)
	}

	if enrichment.ResolvedFiles != 1 {
		t.Fatalf(
			"resolved files = %d, want 1",
			enrichment.ResolvedFiles,
		)
	}

	if enrichment.EnrichedFrames != 2 {
		t.Fatalf(
			"enriched frames = %d, want 2",
			enrichment.EnrichedFrames,
		)
	}

	if enrichment.FailedFiles != 0 {
		t.Fatalf(
			"failed files = %d, want 0",
			enrichment.FailedFiles,
		)
	}

	if resolver.calls["torch_test.py"] != 1 {
		t.Fatalf(
			"resolver calls = %d, want 1",
			resolver.calls["torch_test.py"],
		)
	}

	firstFrame := result.Threads[0].Frames[0]

	if firstFrame.SourcePath != "/root/torch_test.py" {
		t.Fatalf(
			"source path = %q, want %q",
			firstFrame.SourcePath,
			"/root/torch_test.py",
		)
	}

	if firstFrame.SourceText != "torch.cuda.synchronize()" {
		t.Fatalf(
			"source text = %q",
			firstFrame.SourceText,
		)
	}

	secondFrame := result.Threads[0].Frames[1]

	if secondFrame.SourceText != `x = torch.ones(4, device="cuda")` {
		t.Fatalf(
			"source text = %q",
			secondFrame.SourceText,
		)
	}
}

func TestEnrichSourcesResolutionFailureIsNonFatal(t *testing.T) {
	resolver := &fakeSourceResolver{
		errors: map[string]error{
			"missing.py": errors.New(
				"permission denied",
			),
		},
	}

	result := DumpResult{
		ProcessID: 31736,
		Threads: []DumpThread{
			{
				ID: "31736",
				Frames: []DumpFrame{
					{
						Function: "<module>",
						File:     "missing.py",
						Line:     10,
					},
				},
			},
		},
	}

	enrichment, err := EnrichSources(
		context.Background(),
		resolver,
		31736,
		&result,
	)
	if err != nil {
		t.Fatalf(
			"enrich sources: %v",
			err,
		)
	}

	if enrichment.FailedFiles != 1 {
		t.Fatalf(
			"failed files = %d, want 1",
			enrichment.FailedFiles,
		)
	}

	if enrichment.EnrichedFrames != 0 {
		t.Fatalf(
			"enriched frames = %d, want 0",
			enrichment.EnrichedFrames,
		)
	}

	if enrichment.SkippedFrames != 1 {
		t.Fatalf(
			"skipped frames = %d, want 1",
			enrichment.SkippedFrames,
		)
	}

	if len(enrichment.Errors) != 1 {
		t.Fatalf(
			"errors = %d, want 1",
			len(enrichment.Errors),
		)
	}
}

func TestEnrichSourcesSkipsFramesWithoutLocation(t *testing.T) {
	resolver := &fakeSourceResolver{}

	result := DumpResult{
		ProcessID: 31736,
		Threads: []DumpThread{
			{
				ID: "31736",
				Frames: []DumpFrame{
					{
						Function: "native frame",
					},
					{
						Function: "unknown line",
						File:     "torch_test.py",
						Line:     0,
					},
				},
			},
		},
	}

	enrichment, err := EnrichSources(
		context.Background(),
		resolver,
		31736,
		&result,
	)
	if err != nil {
		t.Fatalf(
			"enrich sources: %v",
			err,
		)
	}

	if enrichment.SkippedFrames != 2 {
		t.Fatalf(
			"skipped frames = %d, want 2",
			enrichment.SkippedFrames,
		)
	}

	if len(resolver.calls) != 0 {
		t.Fatalf(
			"resolver calls = %d, want 0",
			len(resolver.calls),
		)
	}
}

func TestEnrichSourcesLineOutOfRange(t *testing.T) {
	resolver := &fakeSourceResolver{
		results: map[string]coresource.SourceFile{
			"train.py": {
				Path: "/workspace/train.py",
				Lines: []coresource.SourceLine{
					{
						Number: 1,
						Text:   "import torch",
					},
				},
			},
		},
	}

	result := DumpResult{
		ProcessID: 31736,
		Threads: []DumpThread{
			{
				ID: "31736",
				Frames: []DumpFrame{
					{
						Function: "<module>",
						File:     "train.py",
						Line:     20,
					},
				},
			},
		},
	}

	enrichment, err := EnrichSources(
		context.Background(),
		resolver,
		31736,
		&result,
	)
	if err != nil {
		t.Fatalf(
			"enrich sources: %v",
			err,
		)
	}

	if enrichment.EnrichedFrames != 0 {
		t.Fatalf(
			"enriched frames = %d, want 0",
			enrichment.EnrichedFrames,
		)
	}

	if enrichment.SkippedFrames != 1 {
		t.Fatalf(
			"skipped frames = %d, want 1",
			enrichment.SkippedFrames,
		)
	}

	if len(enrichment.Errors) != 1 {
		t.Fatalf(
			"errors = %d, want 1",
			len(enrichment.Errors),
		)
	}
}

func TestEnrichSourcesRejectsNilResolver(t *testing.T) {
	result := DumpResult{}

	_, err := EnrichSources(
		context.Background(),
		nil,
		31736,
		&result,
	)

	if err == nil {
		t.Fatal(
			"expected nil resolver error",
		)
	}
}

func TestEnrichSourcesRejectsInvalidPID(t *testing.T) {
	result := DumpResult{}

	_, err := EnrichSources(
		context.Background(),
		&fakeSourceResolver{},
		0,
		&result,
	)

	if err == nil {
		t.Fatal(
			"expected invalid PID error",
		)
	}
}

func TestEnrichSourcesRejectsNilResult(t *testing.T) {
	_, err := EnrichSources(
		context.Background(),
		&fakeSourceResolver{},
		31736,
		nil,
	)

	if err == nil {
		t.Fatal(
			"expected nil result error",
		)
	}
}

func TestEnrichSourcesContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(
		context.Background(),
	)
	cancel()

	result := DumpResult{}

	_, err := EnrichSources(
		ctx,
		&fakeSourceResolver{},
		31736,
		&result,
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
