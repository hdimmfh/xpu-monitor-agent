package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	coresource "github.com/hdimmfh/xpu-monitor-agent/pkg/source"
)

type fakeSourceResolver struct {
	result  coresource.SourceFile
	err     error
	request coresource.ResolveRequest
}

func (f *fakeSourceResolver) Resolve(
	ctx context.Context,
	request coresource.ResolveRequest,
) (coresource.SourceFile, error) {
	if err := ctx.Err(); err != nil {
		return coresource.SourceFile{}, err
	}

	f.request = request

	if f.err != nil {
		return coresource.SourceFile{}, f.err
	}

	return f.result, nil
}

func TestSourceHandlerSuccess(t *testing.T) {
	resolver := &fakeSourceResolver{
		result: coresource.SourceFile{
			PID:         31736,
			Path:        "/root/torch_test.py",
			ContentHash: "sha256:test",
			SizeBytes:   43,
			Lines: []coresource.SourceLine{
				{
					Number: 1,
					Text:   "import torch",
				},
				{
					Number: 2,
					Text:   "torch.cuda.synchronize()",
				},
			},
		},
	}

	handler := newSourceHandler(
		resolver,
	)

	request := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/source?pid=31736&file=torch_test.py",
		nil,
	)

	response := httptest.NewRecorder()

	handler.ServeHTTP(
		response,
		request,
	)

	if response.Code != http.StatusOK {
		t.Fatalf(
			"status = %d, want %d; body=%s",
			response.Code,
			http.StatusOK,
			response.Body.String(),
		)
	}

	if resolver.request.PID != 31736 {
		t.Fatalf(
			"PID = %d, want 31736",
			resolver.request.PID,
		)
	}

	if resolver.request.File != "torch_test.py" {
		t.Fatalf(
			"file = %q, want %q",
			resolver.request.File,
			"torch_test.py",
		)
	}

	var result coresource.SourceFile

	if err := json.Unmarshal(
		response.Body.Bytes(),
		&result,
	); err != nil {
		t.Fatalf(
			"decode response: %v",
			err,
		)
	}

	if result.Path != "/root/torch_test.py" {
		t.Fatalf(
			"path = %q, want %q",
			result.Path,
			"/root/torch_test.py",
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
			"line text = %q",
			result.Lines[1].Text,
		)
	}
}

func TestSourceHandlerMissingPID(t *testing.T) {
	handler := newSourceHandler(
		&fakeSourceResolver{},
	)

	request := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/source?file=torch_test.py",
		nil,
	)

	response := httptest.NewRecorder()

	handler.ServeHTTP(
		response,
		request,
	)

	if response.Code != http.StatusBadRequest {
		t.Fatalf(
			"status = %d, want %d",
			response.Code,
			http.StatusBadRequest,
		)
	}

	if !strings.Contains(
		response.Body.String(),
		"missing required query parameter: pid",
	) {
		t.Fatalf(
			"unexpected body: %s",
			response.Body.String(),
		)
	}
}

func TestSourceHandlerInvalidPID(t *testing.T) {
	handler := newSourceHandler(
		&fakeSourceResolver{},
	)

	request := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/source?pid=invalid&file=torch_test.py",
		nil,
	)

	response := httptest.NewRecorder()

	handler.ServeHTTP(
		response,
		request,
	)

	if response.Code != http.StatusBadRequest {
		t.Fatalf(
			"status = %d, want %d",
			response.Code,
			http.StatusBadRequest,
		)
	}
}

func TestSourceHandlerMissingFile(t *testing.T) {
	handler := newSourceHandler(
		&fakeSourceResolver{},
	)

	request := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/source?pid=31736",
		nil,
	)

	response := httptest.NewRecorder()

	handler.ServeHTTP(
		response,
		request,
	)

	if response.Code != http.StatusBadRequest {
		t.Fatalf(
			"status = %d, want %d",
			response.Code,
			http.StatusBadRequest,
		)
	}

	if !strings.Contains(
		response.Body.String(),
		"missing required query parameter: file",
	) {
		t.Fatalf(
			"unexpected body: %s",
			response.Body.String(),
		)
	}
}

func TestSourceHandlerMethodNotAllowed(t *testing.T) {
	handler := newSourceHandler(
		&fakeSourceResolver{},
	)

	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/source?pid=31736&file=torch_test.py",
		nil,
	)

	response := httptest.NewRecorder()

	handler.ServeHTTP(
		response,
		request,
	)

	if response.Code != http.StatusMethodNotAllowed {
		t.Fatalf(
			"status = %d, want %d",
			response.Code,
			http.StatusMethodNotAllowed,
		)
	}

	if response.Header().Get("Allow") != http.MethodGet {
		t.Fatalf(
			"Allow = %q, want %q",
			response.Header().Get("Allow"),
			http.MethodGet,
		)
	}
}

func TestSourceHandlerResolverFailure(t *testing.T) {
	handler := newSourceHandler(
		&fakeSourceResolver{
			err: errors.New(
				"source file not found",
			),
		},
	)

	request := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/source?pid=31736&file=missing.py",
		nil,
	)

	response := httptest.NewRecorder()

	handler.ServeHTTP(
		response,
		request,
	)

	if response.Code != http.StatusNotFound {
		t.Fatalf(
			"status = %d, want %d",
			response.Code,
			http.StatusNotFound,
		)
	}

	if !strings.Contains(
		response.Body.String(),
		"source file not found",
	) {
		t.Fatalf(
			"unexpected body: %s",
			response.Body.String(),
		)
	}
}

func TestSourceHandlerNilResolver(t *testing.T) {
	handler := newSourceHandler(nil)

	request := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/source?pid=31736&file=torch_test.py",
		nil,
	)

	response := httptest.NewRecorder()

	handler.ServeHTTP(
		response,
		request,
	)

	if response.Code != http.StatusInternalServerError {
		t.Fatalf(
			"status = %d, want %d",
			response.Code,
			http.StatusInternalServerError,
		)
	}
}
