package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	promexporter "github.com/hdimmfh/xpu-monitor-agent/exporters/prometheus"
	"github.com/hdimmfh/xpu-monitor-agent/pkg/collector"
	coresource "github.com/hdimmfh/xpu-monitor-agent/pkg/source"
	"github.com/hdimmfh/xpu-monitor-agent/plugins/host"
	"github.com/hdimmfh/xpu-monitor-agent/plugins/nvidia"
	prom "github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const (
	defaultListenAddress   = ":9108"
	defaultMetricsPath     = "/metrics"
	defaultCollectTimeout  = 10 * time.Second
	defaultShutdownTimeout = 5 * time.Second
)

type apiErrorResponse struct {
	Error string `json:"error"`
}

// runServe starts the XPUMON Prometheus exporter and source API server.
func runServe(
	ctx context.Context,
	args []string,
) error {
	flags := flag.NewFlagSet(
		"serve",
		flag.ContinueOnError,
	)

	listenAddress := flags.String(
		"listen-address",
		defaultListenAddress,
		"address on which to expose XPUMON HTTP endpoints",
	)

	metricsPath := flags.String(
		"metrics-path",
		defaultMetricsPath,
		"HTTP path on which to expose Prometheus metrics",
	)

	collectTimeout := flags.Duration(
		"collect-timeout",
		defaultCollectTimeout,
		"maximum duration allowed for one metric collection",
	)

	shutdownTimeout := flags.Duration(
		"shutdown-timeout",
		defaultShutdownTimeout,
		"maximum duration allowed for graceful shutdown",
	)

	if err := flags.Parse(args); err != nil {
		return err
	}

	if strings.TrimSpace(*listenAddress) == "" {
		return errors.New(
			"--listen-address must not be empty",
		)
	}

	if strings.TrimSpace(*metricsPath) == "" {
		return errors.New(
			"--metrics-path must not be empty",
		)
	}

	if !strings.HasPrefix(
		*metricsPath,
		"/",
	) {
		return fmt.Errorf(
			"--metrics-path must start with '/': %q",
			*metricsPath,
		)
	}

	if *collectTimeout <= 0 {
		return errors.New(
			"--collect-timeout must be greater than zero",
		)
	}

	if *shutdownTimeout <= 0 {
		return errors.New(
			"--shutdown-timeout must be greater than zero",
		)
	}

	hostPlugin := host.New()

	nvidiaPlugin, err := nvidia.New()
	if err != nil {
		return fmt.Errorf(
			"create NVIDIA plugin: %w",
			err,
		)
	}

	defer func() {
		if closeErr := nvidiaPlugin.Close(); closeErr != nil {
			log.Printf(
				"close NVIDIA plugin: %v",
				closeErr,
			)
		}
	}()

	metricCollector := collector.New(
		hostPlugin,
		nvidiaPlugin,
	)

	xpumonCollector := promexporter.New(
		metricCollector,
		*collectTimeout,
	)

	registry := prom.NewRegistry()

	if err := registry.Register(xpumonCollector); err != nil {
		return fmt.Errorf(
			"register XPUMON Prometheus collector: %w",
			err,
		)
	}

	sourceResolver := coresource.NewLinuxResolver()

	handler := newServeHandler(
		registry,
		*metricsPath,
		sourceResolver,
	)

	server := &http.Server{
		Addr:              *listenAddress,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	serverError := make(
		chan error,
		1,
	)

	go func() {
		log.Printf(
			"XPUMON server listening address=%s metrics_path=%s source_path=/api/v1/source",
			*listenAddress,
			*metricsPath,
		)

		err := server.ListenAndServe()

		if err != nil &&
			!errors.Is(
				err,
				http.ErrServerClosed,
			) {
			serverError <- err
			return
		}

		serverError <- nil
	}()

	select {
	case err := <-serverError:
		if err != nil {
			return fmt.Errorf(
				"serve XPUMON HTTP endpoints: %w",
				err,
			)
		}

		return nil

	case <-ctx.Done():
		log.Printf(
			"shutting down XPUMON HTTP server",
		)
	}

	shutdownContext, cancel := context.WithTimeout(
		context.Background(),
		*shutdownTimeout,
	)
	defer cancel()

	if err := server.Shutdown(shutdownContext); err != nil {
		if closeErr := server.Close(); closeErr != nil {
			log.Printf(
				"force close HTTP server: %v",
				closeErr,
			)
		}

		return fmt.Errorf(
			"shutdown XPUMON HTTP server: %w",
			err,
		)
	}

	if err := <-serverError; err != nil {
		return fmt.Errorf(
			"serve XPUMON HTTP endpoints: %w",
			err,
		)
	}

	return nil
}

func newServeHandler(
	registry *prom.Registry,
	metricsPath string,
	sourceResolver coresource.Resolver,
) http.Handler {
	mux := http.NewServeMux()

	mux.Handle(
		metricsPath,
		promhttp.HandlerFor(
			registry,
			promhttp.HandlerOpts{
				ErrorHandling: promhttp.ContinueOnError,
			},
		),
	)

	mux.Handle(
		"/api/v1/source",
		newSourceHandler(
			sourceResolver,
		),
	)

	mux.HandleFunc(
		"/healthz",
		handleHealth,
	)

	mux.HandleFunc(
		"/",
		func(
			writer http.ResponseWriter,
			request *http.Request,
		) {
			handleIndex(
				writer,
				request,
				metricsPath,
			)
		},
	)

	return mux
}

func newSourceHandler(
	resolver coresource.Resolver,
) http.Handler {
	return http.HandlerFunc(
		func(
			writer http.ResponseWriter,
			request *http.Request,
		) {
			if request.Method != http.MethodGet {
				writer.Header().Set(
					"Allow",
					http.MethodGet,
				)

				writeHTTPJSON(
					writer,
					http.StatusMethodNotAllowed,
					apiErrorResponse{
						Error: "method not allowed",
					},
				)

				return
			}

			if resolver == nil {
				writeHTTPJSON(
					writer,
					http.StatusInternalServerError,
					apiErrorResponse{
						Error: "source resolver is unavailable",
					},
				)

				return
			}

			pidValue := strings.TrimSpace(
				request.URL.Query().Get("pid"),
			)

			if pidValue == "" {
				writeHTTPJSON(
					writer,
					http.StatusBadRequest,
					apiErrorResponse{
						Error: "missing required query parameter: pid",
					},
				)

				return
			}

			pid, err := strconv.Atoi(pidValue)
			if err != nil || pid <= 0 {
				writeHTTPJSON(
					writer,
					http.StatusBadRequest,
					apiErrorResponse{
						Error: fmt.Sprintf(
							"invalid pid %q: must be a positive integer",
							pidValue,
						),
					},
				)

				return
			}

			file := strings.TrimSpace(
				request.URL.Query().Get("file"),
			)

			if file == "" {
				writeHTTPJSON(
					writer,
					http.StatusBadRequest,
					apiErrorResponse{
						Error: "missing required query parameter: file",
					},
				)

				return
			}

			result, err := resolver.Resolve(
				request.Context(),
				coresource.ResolveRequest{
					PID:  pid,
					File: file,
				},
			)
			if err != nil {
				statusCode := sourceResolveStatusCode(err)

				writeHTTPJSON(
					writer,
					statusCode,
					apiErrorResponse{
						Error: err.Error(),
					},
				)

				return
			}

			writeHTTPJSON(
				writer,
				http.StatusOK,
				result,
			)
		},
	)
}

func sourceResolveStatusCode(
	err error,
) int {
	if err == nil {
		return http.StatusOK
	}

	if errors.Is(
		err,
		context.Canceled,
	) {
		return http.StatusRequestTimeout
	}

	if errors.Is(
		err,
		context.DeadlineExceeded,
	) {
		return http.StatusGatewayTimeout
	}

	// Resolver errors currently wrap filesystem errors with %w.
	// Do not expose whether the failure was permission-related or whether
	// a specific host path exists through separate HTTP status semantics.
	return http.StatusNotFound
}

func handleHealth(
	writer http.ResponseWriter,
	request *http.Request,
) {
	if request.Method != http.MethodGet {
		writer.Header().Set(
			"Allow",
			http.MethodGet,
		)

		http.Error(
			writer,
			http.StatusText(
				http.StatusMethodNotAllowed,
			),
			http.StatusMethodNotAllowed,
		)

		return
	}

	writer.Header().Set(
		"Content-Type",
		"text/plain; charset=utf-8",
	)

	writer.WriteHeader(
		http.StatusOK,
	)

	if _, err := writer.Write(
		[]byte("ok\n"),
	); err != nil {
		log.Printf(
			"write health response: %v",
			err,
		)
	}
}

func handleIndex(
	writer http.ResponseWriter,
	request *http.Request,
	metricsPath string,
) {
	if request.URL.Path != "/" {
		http.NotFound(
			writer,
			request,
		)

		return
	}

	if request.Method != http.MethodGet {
		writer.Header().Set(
			"Allow",
			http.MethodGet,
		)

		http.Error(
			writer,
			http.StatusText(
				http.StatusMethodNotAllowed,
			),
			http.StatusMethodNotAllowed,
		)

		return
	}

	writer.Header().Set(
		"Content-Type",
		"text/html; charset=utf-8",
	)

	_, err := fmt.Fprintf(
		writer,
		`<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <title>XPUMON</title>
</head>
<body>
  <h1>XPUMON</h1>
  <ul>
    <li><a href="%s">Prometheus metrics</a></li>
    <li><a href="/healthz">Health</a></li>
    <li>Source API: <code>/api/v1/source?pid=&lt;PID&gt;&amp;file=&lt;PATH&gt;</code></li>
  </ul>
</body>
</html>
`,
		metricsPath,
	)
	if err != nil {
		log.Printf(
			"write index response: %v",
			err,
		)
	}
}

func writeHTTPJSON(
	writer http.ResponseWriter,
	statusCode int,
	value any,
) {
	writer.Header().Set(
		"Content-Type",
		"application/json; charset=utf-8",
	)

	writer.Header().Set(
		"Cache-Control",
		"no-store",
	)

	writer.WriteHeader(
		statusCode,
	)

	encoder := json.NewEncoder(writer)
	encoder.SetEscapeHTML(false)

	if err := encoder.Encode(value); err != nil {
		log.Printf(
			"encode HTTP JSON response: %v",
			err,
		)
	}
}
