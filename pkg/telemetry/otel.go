// Package telemetry provides OpenTelemetry tracing and Prometheus metrics setup.
package telemetry

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"

	"erg.ninja/pkg/config"
	"erg.ninja/pkg/logger"
)

// Tracer is the package-level tracer instance.
var Tracer trace.Tracer

// TracerProvider holds the OpenTelemetry tracer provider with shutdown support.
type TracerProvider struct {
	provider *sdktrace.TracerProvider
	log      *logger.Logger
}

// NewTracerProvider creates an OpenTelemetry TracerProvider with stdout (or Jaeger) export.
// It automatically instruments HTTP and gRPC if the OpenTelemetry instrumentation libraries are imported.
func NewTracerProvider(ctx context.Context, cfg config.TelemetryConfig, log *logger.Logger) (*TracerProvider, error) {
	if !cfg.Enabled {
		// Return a no-op provider.
		tp := &TracerProvider{log: log}
		Tracer = otel.Tracer(cfg.ServiceName)
		return tp, nil
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(cfg.ServiceName),
			semconv.ServiceVersion(cfg.ServiceVersion),
			semconv.DeploymentEnvironment(cfg.Environment),
		),
		resource.WithHost(),
		resource.WithProcess(),
	)
	if err != nil {
		return nil, fmt.Errorf("telemetry: create resource: %w", err)
	}

	var exporter sdktrace.SpanExporter
	var errMsg string

	// Try stdout exporter first (always available).
	// In production, swap this for the Jaeger exporter.
	stdExporter, err := stdouttrace.New(stdouttrace.WithPrettyPrint())
	if err != nil {
		errMsg = fmt.Sprintf("stdouttrace: %v", err)
	} else {
		exporter = stdExporter
	}

	if exporter == nil {
		return nil, fmt.Errorf("telemetry: no exporter available: %s", errMsg)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter,
			sdktrace.WithBatchTimeout(5*time.Second),
			sdktrace.WithMaxExportBatchSize(512),
		),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
	)

	otel.SetTracerProvider(tp)
	Tracer = tp.Tracer(cfg.ServiceName)

	// Set up W3C trace context propagation.
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	log.Info().
		Str("service", cfg.ServiceName).
		Str("version", cfg.ServiceVersion).
		Str("environment", cfg.Environment).
		Msg("telemetry: tracer provider initialized")

	return &TracerProvider{provider: tp, log: log}, nil
}

// Tracer returns the tracer for this service.
func (tp *TracerProvider) Tracer() trace.Tracer {
	if tp == nil || tp.provider == nil {
		return otel.Tracer("erg-service")
	}
	return tp.provider.Tracer("")
}

// Shutdown gracefully shuts down the tracer provider.
func (tp *TracerProvider) Shutdown(ctx context.Context) error {
	if tp == nil || tp.provider == nil {
		return nil
	}
	return tp.provider.Shutdown(ctx)
}

// StartSpan starts a new span with the given name and options.
func StartSpan(ctx context.Context, name string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	if Tracer == nil {
		return ctx, trace.SpanFromContext(ctx)
	}
	return Tracer.Start(ctx, name, opts...)
}

// SpanFromContext returns the current span from the context.
func SpanFromContext(ctx context.Context) trace.Span {
	return trace.SpanFromContext(ctx)
}

// AddSpanEvent adds an event to the current span.
func AddSpanEvent(ctx context.Context, name string, attrs ...attribute.KeyValue) {
	span := trace.SpanFromContext(ctx)
	span.AddEvent(name, trace.WithAttributes(attrs...))
}

// SetSpanAttributes sets attributes on the current span.
func SetSpanAttributes(ctx context.Context, attrs ...attribute.KeyValue) {
	span := trace.SpanFromContext(ctx)
	span.SetAttributes(attrs...)
}

// TraceIDFromContext returns the trace ID from the context, or empty string.
func TraceIDFromContext(ctx context.Context) string {
	span := trace.SpanFromContext(ctx)
	if span.SpanContext().IsValid() {
		return span.SpanContext().TraceID().String()
	}
	return ""
}

// SpanIDFromContext returns the span ID from the context, or empty string.
func SpanIDFromContext(ctx context.Context) string {
	span := trace.SpanFromContext(ctx)
	if span.SpanContext().IsValid() {
		return span.SpanContext().SpanID().String()
	}
	return ""
}

// WithCorrelationID adds a correlation ID attribute to the current span.
func WithCorrelationID(ctx context.Context, correlationID string) {
	AddSpanEvent(ctx, "correlation", attribute.String("correlation.id", correlationID))
}
