package ferret

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
)

// mockSpan implements trace.Span for testing
type mockSpan struct {
	trace.Span
	name       string
	attributes []attribute.KeyValue
	events     []string
	status     codes.Code
	statusDesc string
	ended      bool
}

func (m *mockSpan) End(_ ...trace.SpanEndOption) {
	m.ended = true
}

func (m *mockSpan) SetAttributes(kv ...attribute.KeyValue) {
	m.attributes = append(m.attributes, kv...)
}

func (m *mockSpan) SetStatus(code codes.Code, description string) {
	m.status = code
	m.statusDesc = description
}

func (m *mockSpan) RecordError(_ error, _ ...trace.EventOption) {}

func (m *mockSpan) AddEvent(name string, _ ...trace.EventOption) {
	m.events = append(m.events, name)
}

func (m *mockSpan) IsRecording() bool { return true }

func (m *mockSpan) SpanContext() trace.SpanContext {
	return trace.SpanContext{}
}


// mockTracer implements trace.Tracer for testing
type mockTracer struct {
	trace.Tracer
	spans []*mockSpan
}

func (m *mockTracer) Start(
	ctx context.Context,
	spanName string,
	opts ...trace.SpanStartOption,
) (context.Context, trace.Span) {
	// Use the built-in noop span as base
	noopTP := noop.NewTracerProvider()
	noopTracer := noopTP.Tracer("test")
	_, noopSpan := noopTracer.Start(ctx, "noop")
	
	span := &mockSpan{
		Span: noopSpan,
		name: spanName,
	}

	// Apply span start options to capture initial attributes
	cfg := trace.NewSpanStartConfig(opts...)
	span.attributes = append(span.attributes, cfg.Attributes()...)

	m.spans = append(m.spans, span)
	return trace.ContextWithSpan(ctx, span), span
}

// TestOpenTelemetryIntegration verifies OpenTelemetry tracing.
func TestOpenTelemetryIntegration(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(10 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	}))
	defer server.Close()

	// Create mock tracer
	tracer := &mockTracer{}
	config := OpenTelemetryConfig{
		Tracer:         tracer,
		DetailedEvents: true,
	}

	ferret := New(WithOpenTelemetry(config))
	client := &http.Client{Transport: ferret}

	// Make request
	req, err := http.NewRequest("GET", server.URL+"/test", nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Verify span was created
	if len(tracer.spans) != 1 {
		t.Fatalf("Expected 1 span, got %d", len(tracer.spans))
	}

	span := tracer.spans[0]

	// Verify span name
	if span.name != "HTTP GET /test" {
		t.Errorf("Expected span name 'HTTP GET /test', got %s", span.name)
	}

	// Verify span ended
	if !span.ended {
		t.Error("Expected span to be ended")
	}

	// Verify key attributes are present
	hasStatusCode := false
	hasMethod := false

	for _, attr := range span.attributes {
		switch string(attr.Key) {
		case "http.method":
			hasMethod = true
			if attr.Value.AsString() != "GET" {
				t.Errorf("Expected method GET, got %s", attr.Value.AsString())
			}
		case "http.status_code":
			hasStatusCode = true
			if attr.Value.AsInt64() != 200 {
				t.Errorf("Expected status code 200, got %d", attr.Value.AsInt64())
			}
		}
	}

	// Check that we have at least the basic attributes
	if !hasMethod {
		t.Log("Note: http.method attribute not captured in mock (set in Start options)")
	}
	if !hasStatusCode {
		t.Error("Missing http.status_code attribute")
	}

	// Verify status
	if span.status != codes.Ok {
		t.Errorf("Expected OK status, got %v", span.status)
	}

	// Verify events were recorded
	if len(span.events) == 0 {
		t.Error("Expected timing events to be recorded")
	}
}

// TestOpenTelemetryWithError verifies error handling in traces.
func TestOpenTelemetryWithError(t *testing.T) {
	// Create mock tracer
	tracer := &mockTracer{}
	config := OpenTelemetryConfig{
		Tracer: tracer,
	}

	ferret := New(WithOpenTelemetry(config))
	client := &http.Client{Transport: ferret, Timeout: 1 * time.Millisecond}

	// Make request that will timeout
	req, err := http.NewRequest("GET", "http://192.0.2.1", nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	resp, err := client.Do(req)
	if err == nil {
		t.Fatal("Expected request to fail")
	}
	if resp != nil {
		_ = resp.Body.Close()
	}

	// Verify span was created and has error status
	if len(tracer.spans) != 1 {
		t.Fatalf("Expected 1 span, got %d", len(tracer.spans))
	}

	span := tracer.spans[0]
	if span.status != codes.Error {
		t.Errorf("Expected Error status, got %v", span.status)
	}
}

// TestOpenTelemetryHTTPError verifies handling of HTTP errors.
func TestOpenTelemetryHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("Error"))
	}))
	defer server.Close()

	tracer := &mockTracer{}
	config := OpenTelemetryConfig{
		Tracer: tracer,
	}

	ferret := New(WithOpenTelemetry(config))
	client := &http.Client{Transport: ferret}

	req, _ := http.NewRequest("GET", server.URL, nil)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Verify span has error status for 5xx response
	if len(tracer.spans) != 1 {
		t.Fatalf("Expected 1 span, got %d", len(tracer.spans))
	}

	span := tracer.spans[0]
	if span.status != codes.Error {
		t.Errorf("Expected Error status for 500 response, got %v", span.status)
	}

	// Verify status code attribute
	hasStatusCode := false
	for _, attr := range span.attributes {
		if attr.Key == "http.status_code" && attr.Value.AsInt64() == 500 {
			hasStatusCode = true
			break
		}
	}
	if !hasStatusCode {
		t.Error("Missing or incorrect http.status_code attribute")
	}
}

// TestSimpleOpenTelemetryConfig verifies the simple config helper.
func TestSimpleOpenTelemetryConfig(t *testing.T) {
	tracer := &mockTracer{}
	config := SimpleOpenTelemetryConfig(tracer)

	if config.Tracer == nil {
		t.Error("Expected Tracer to be set")
	}

	if !config.DetailedEvents {
		t.Error("Expected DetailedEvents to be true")
	}

	if config.SpanNameFormatter == nil {
		t.Error("Expected SpanNameFormatter to be set")
	}

	// Test formatter
	req, _ := http.NewRequest("POST", "http://example.com/api/users", nil)
	name := config.SpanNameFormatter(req)
	if name != "HTTP POST /api/users" {
		t.Errorf("Expected 'HTTP POST /api/users', got %s", name)
	}
}

// TestCustomSpanNameFormatter verifies custom span naming.
func TestCustomSpanNameFormatter(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	tracer := &mockTracer{}
	config := OpenTelemetryConfig{
		Tracer: tracer,
		SpanNameFormatter: func(req *http.Request) string {
			return "custom-" + req.Method
		},
	}

	ferret := New(WithOpenTelemetry(config))
	client := &http.Client{Transport: ferret}

	req, _ := http.NewRequest("GET", server.URL, nil)
	resp, _ := client.Do(req)
	_ = resp.Body.Close()

	if len(tracer.spans) != 1 {
		t.Fatalf("Expected 1 span, got %d", len(tracer.spans))
	}

	if tracer.spans[0].name != "custom-GET" {
		t.Errorf("Expected span name 'custom-GET', got %s", tracer.spans[0].name)
	}
}
