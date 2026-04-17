// Package tracing wires OpenTelemetry into the Hive server. It deliberately
// stays a thin shim over the SDK: Setup() returns a shutdown func so callers
// don't have to know which protocol/exporter is in use, and the tracer name
// is the Hive service identifier so spans show up grouped in any OTLP
// collector.
//
// When OTEL_EXPORTER_OTLP_ENDPOINT is empty and cfg.Enabled is false, Setup()
// installs a no-op tracer provider so all `otel.Tracer(...).Start(ctx, ...)`
// calls still return a usable (ctx, span) pair without talking to any
// collector. This keeps instrumented code paths free of "is tracing on?"
// branches.
package tracing

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

// ServiceName is the default name stamped on every exported span.
const ServiceName = "hive"

// Config controls exporter selection + sampling.
type Config struct {
	// Enabled forces tracing on even when OTEL_EXPORTER_OTLP_ENDPOINT is unset
	// (useful for stdout debugging via a custom exporter; future hook).
	Enabled bool
	// Endpoint overrides OTEL_EXPORTER_OTLP_ENDPOINT. Empty = env var wins.
	Endpoint string
	// Protocol picks grpc (default) or http/protobuf. Empty = grpc.
	Protocol string
	// SampleRatio: 1.0 = everything, 0.0 = nothing, any in between = fraction.
	// Defaults to 1.0 when unset.
	SampleRatio float64
	// ServiceVersion stamps the service.version resource attribute — handy
	// for correlating traces with a deploy.
	ServiceVersion string
}

// Setup initialises a global tracer provider. Call the returned shutdown
// function on server exit to flush pending spans. Safe to call even when
// cfg.Enabled is false — it installs a no-op exporter and returns a no-op
// shutdown.
//
// Standard OTel env vars are respected when the YAML config leaves fields
// blank: OTEL_EXPORTER_OTLP_ENDPOINT and OTEL_EXPORTER_OTLP_PROTOCOL. This
// lets operators wire tracing via env in container deployments without
// mounting a custom hive.yaml.
func Setup(ctx context.Context, cfg Config) (func(context.Context) error, error) {
	if cfg.Endpoint == "" {
		cfg.Endpoint = os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	}
	if cfg.Protocol == "" {
		cfg.Protocol = os.Getenv("OTEL_EXPORTER_OTLP_PROTOCOL")
	}
	if !cfg.Enabled && cfg.Endpoint == "" {
		// Nothing to do — otel already ships a noop TracerProvider by default,
		// which means every Tracer() call returns a usable no-op span. We
		// still install a propagator so incoming trace context is preserved
		// across federation hops.
		otel.SetTextMapPropagator(propagation.TraceContext{})
		slog.Info("tracing: disabled (no OTLP endpoint configured)")
		return func(context.Context) error { return nil }, nil
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(ServiceName),
			semconv.ServiceVersion(coalesce(cfg.ServiceVersion, "dev")),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("building resource: %w", err)
	}

	exporter, err := buildExporter(ctx, cfg)
	if err != nil {
		return nil, err
	}

	ratio := cfg.SampleRatio
	if ratio <= 0 {
		ratio = 1.0
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter, sdktrace.WithBatchTimeout(5*time.Second)),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.TraceIDRatioBased(ratio)),
	)
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.TraceContext{})
	slog.Info("tracing: enabled",
		"endpoint", cfg.Endpoint,
		"protocol", coalesce(cfg.Protocol, "grpc"),
		"sample_ratio", ratio)

	return func(shutdownCtx context.Context) error {
		// 5s budget covers "flush everything then stop" for sane batch sizes.
		ctx, cancel := context.WithTimeout(shutdownCtx, 5*time.Second)
		defer cancel()
		return tp.Shutdown(ctx)
	}, nil
}

// buildExporter picks an exporter per cfg.Protocol. grpc is the
// recommended default (lower overhead, streaming); http/protobuf is useful
// when the collector is behind an HTTP-only ingress.
func buildExporter(ctx context.Context, cfg Config) (sdktrace.SpanExporter, error) {
	switch cfg.Protocol {
	case "", "grpc":
		opts := []otlptracegrpc.Option{otlptracegrpc.WithInsecure()}
		if cfg.Endpoint != "" {
			opts = append(opts, otlptracegrpc.WithEndpoint(cfg.Endpoint))
		}
		return otlptrace.New(ctx, otlptracegrpc.NewClient(opts...))
	case "http", "http/protobuf":
		opts := []otlptracehttp.Option{otlptracehttp.WithInsecure()}
		if cfg.Endpoint != "" {
			opts = append(opts, otlptracehttp.WithEndpoint(cfg.Endpoint))
		}
		return otlptrace.New(ctx, otlptracehttp.NewClient(opts...))
	default:
		return nil, errors.New("unknown OTLP protocol: " + cfg.Protocol + " (use grpc or http)")
	}
}

func coalesce(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}
