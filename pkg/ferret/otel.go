package ferret

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// OpenTelemetryConfig holds configuration for OpenTelemetry tracing.
type OpenTelemetryConfig struct {
	// Tracer to use for creating spans
	Tracer trace.Tracer
	
	// SpanNameFormatter allows customizing the span name
	SpanNameFormatter func(*http.Request) string
	
	// Whether to record detailed timing events
	DetailedEvents bool
}

// WithOpenTelemetry returns an option that enables OpenTelemetry tracing.
func WithOpenTelemetry(config OpenTelemetryConfig) Option {
	// Set default span name formatter if not provided
	if config.SpanNameFormatter == nil {
		config.SpanNameFormatter = func(req *http.Request) string {
			return fmt.Sprintf("HTTP %s %s", req.Method, req.URL.Path)
		}
	}

	return func(f *Ferret) {
		// Wrap the existing transport with OpenTelemetry instrumentation
		f.next = &otelTransport{
			next:   f.next,
			config: config,
			ferret: f,
		}
	}
}

// otelTransport wraps a RoundTripper to collect OpenTelemetry traces.
type otelTransport struct {
	next   http.RoundTripper
	config OpenTelemetryConfig
	ferret *Ferret
}

// RoundTrip implements http.RoundTripper with OpenTelemetry tracing.
func (t *otelTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Start a new span
	ctx, span := t.config.Tracer.Start(req.Context(), t.config.SpanNameFormatter(req),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("http.method", req.Method),
			attribute.String("http.url", req.URL.String()),
			attribute.String("http.scheme", req.URL.Scheme),
			attribute.String("net.peer.name", req.URL.Host),
		),
	)
	defer span.End()

	// Update request with new context
	req = req.WithContext(ctx)

	// Execute the request
	resp, err := t.next.RoundTrip(req)

	// Get the result from the response if available
	var result *Result
	if resp != nil && resp.Request != nil {
		result = GetResult(resp.Request)
	}

	// Set span attributes based on response
	if resp != nil {
		span.SetAttributes(
			attribute.Int("http.status_code", resp.StatusCode),
			attribute.String("http.status_text", http.StatusText(resp.StatusCode)),
		)
		
		// Set span status based on HTTP status code
		if resp.StatusCode >= 400 {
			span.SetStatus(codes.Error, http.StatusText(resp.StatusCode))
		} else {
			span.SetStatus(codes.Ok, "")
		}
	}

	// Handle errors
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		span.SetAttributes(
			attribute.String("error.type", fmt.Sprintf("%T", err)),
		)
	}

	// Record timing information if available
	if result != nil && t.config.DetailedEvents {
		// Add timing attributes
		span.SetAttributes(
			attribute.Float64("http.duration_ms", float64(result.TotalDuration().Milliseconds())),
			attribute.Float64("http.dns_duration_ms", float64(result.DNSDuration().Milliseconds())),
			attribute.Float64("http.connect_duration_ms", float64(result.ConnectionDuration().Milliseconds())),
			attribute.Float64("http.tls_duration_ms", float64(result.TLSDuration().Milliseconds())),
			attribute.Float64("http.ttfb_ms", float64(result.TTFB().Milliseconds())),
			attribute.Float64("http.server_duration_ms", float64(result.ServerProcessingDuration().Milliseconds())),
			attribute.Float64("http.transfer_duration_ms", float64(result.DataTransferDuration().Milliseconds())),
		)

		// Add timing events
		if !result.DNSStart.IsZero() {
			span.AddEvent("dns.start", trace.WithTimestamp(result.DNSStart))
		}
		if !result.DNSDone.IsZero() {
			span.AddEvent("dns.done", trace.WithTimestamp(result.DNSDone))
		}
		if !result.ConnectStart.IsZero() {
			span.AddEvent("connect.start", trace.WithTimestamp(result.ConnectStart))
		}
		if !result.TLSHandshakeStart.IsZero() {
			span.AddEvent("tls.start", trace.WithTimestamp(result.TLSHandshakeStart))
		}
		if !result.TLSHandshakeDone.IsZero() {
			span.AddEvent("tls.done", trace.WithTimestamp(result.TLSHandshakeDone))
		}
		if !result.ConnectDone.IsZero() {
			span.AddEvent("connect.done", trace.WithTimestamp(result.ConnectDone))
		}
		if !result.FirstByte.IsZero() {
			span.AddEvent("first_byte", trace.WithTimestamp(result.FirstByte))
		}
		if !result.End.IsZero() {
			span.AddEvent("request.done", trace.WithTimestamp(result.End))
		}
	}

	return resp, err
}

// contextWithSpan is a helper to inject a span into a context for testing.
func contextWithSpan(ctx context.Context, span trace.Span) context.Context {
	return trace.ContextWithSpan(ctx, span)
}

// SimpleOpenTelemetryConfig creates a simple OpenTelemetry configuration.
func SimpleOpenTelemetryConfig(tracer trace.Tracer) OpenTelemetryConfig {
	return OpenTelemetryConfig{
		Tracer:         tracer,
		DetailedEvents: true,
		SpanNameFormatter: func(req *http.Request) string {
			return fmt.Sprintf("HTTP %s %s", req.Method, req.URL.Path)
		},
	}
}

// WithSimpleOpenTelemetry is a convenience option that sets up OpenTelemetry with sensible defaults.
func WithSimpleOpenTelemetry(tracer trace.Tracer) Option {
	return WithOpenTelemetry(SimpleOpenTelemetryConfig(tracer))
}

// ExtractSpanContext extracts the span context from an HTTP request.
// This is useful for propagating trace context across service boundaries.
func ExtractSpanContext(req *http.Request) trace.SpanContext {
	return trace.SpanContextFromContext(req.Context())
}

// InjectSpanContext injects a span context into an HTTP request.
// This is useful for propagating trace context across service boundaries.
func InjectSpanContext(req *http.Request, sc trace.SpanContext) {
	// This would typically use the OpenTelemetry propagator API
	// For now, we'll just document that users should use the propagator
	// Example:
	// propagator := propagation.TraceContext{}
	// propagator.Inject(ctx, propagation.HeaderCarrier(req.Header))
}

// spanStatusFromHTTPStatus converts an HTTP status code to an OpenTelemetry status.
func spanStatusFromHTTPStatus(statusCode int) (codes.Code, string) {
	if statusCode < 400 {
		return codes.Ok, ""
	}
	return codes.Error, strconv.Itoa(statusCode)
}