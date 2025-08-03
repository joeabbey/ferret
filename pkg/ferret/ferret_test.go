package ferret

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

// TestConcurrentRequests verifies that Ferret is safe for concurrent use.
func TestConcurrentRequests(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(10 * time.Millisecond) // Simulate some processing
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	defer server.Close()

	// Create a Ferret transport
	ferret := New()
	client := &http.Client{Transport: ferret}

	// Number of concurrent requests
	concurrency := 10
	var wg sync.WaitGroup
	results := make([]*Result, concurrency)
	errors := make([]error, concurrency)

	// Execute concurrent requests
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()

			req, err := http.NewRequest("GET", server.URL, nil)
			if err != nil {
				errors[index] = err
				return
			}

			resp, err := client.Do(req)
			if err != nil {
				errors[index] = err
				return
			}
			defer resp.Body.Close()

			// Get the result from the response's request
			results[index] = GetResult(resp.Request)
		}(i)
	}

	wg.Wait()

	// Verify results
	for i := 0; i < concurrency; i++ {
		if errors[i] != nil {
			t.Errorf("Request %d failed: %v", i, errors[i])
			continue
		}

		if results[i] == nil {
			t.Errorf("Request %d: no result found", i)
			continue
		}

		// Verify timing data
		if results[i].ConnectionDuration() <= 0 {
			t.Errorf("Request %d: invalid connection duration: %v", i, results[i].ConnectionDuration())
		}

		if results[i].TotalDuration() <= 0 {
			t.Errorf("Request %d: invalid total duration: %v", i, results[i].TotalDuration())
		}

		if results[i].TotalDuration() < results[i].ConnectionDuration() {
			t.Errorf("Request %d: total duration (%v) < connection duration (%v)",
				i, results[i].TotalDuration(), results[i].ConnectionDuration())
		}
	}
}

// TestOptions verifies that options work correctly.
func TestOptions(t *testing.T) {
	tests := []struct {
		name    string
		options []Option
		check   func(*testing.T, *Ferret)
	}{
		{
			name:    "WithKeepAlives(true)",
			options: []Option{WithKeepAlives(true)},
			check: func(t *testing.T, f *Ferret) {
				if f.disableKeepAlives {
					t.Error("Expected keep-alives to be enabled")
				}
			},
		},
		{
			name:    "WithKeepAlives(false)",
			options: []Option{WithKeepAlives(false)},
			check: func(t *testing.T, f *Ferret) {
				if !f.disableKeepAlives {
					t.Error("Expected keep-alives to be disabled")
				}
			},
		},
		{
			name:    "WithTimeout",
			options: []Option{WithTimeout(5*time.Second, 0)},
			check: func(t *testing.T, f *Ferret) {
				if f.dialer.Timeout != 5*time.Second {
					t.Errorf("Expected timeout 5s, got %v", f.dialer.Timeout)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := New(tt.options...)
			tt.check(t, f)
		})
	}
}

// TestResultMethods verifies Result methods work correctly.
func TestResultMethods(t *testing.T) {
	now := time.Now()
	r := &Result{
		Start:       now,
		ConnectDone: now.Add(100 * time.Millisecond),
		FirstByte:   now.Add(150 * time.Millisecond),
		End:         now.Add(200 * time.Millisecond),
	}

	if r.ConnectionDuration() != 100*time.Millisecond {
		t.Errorf("Expected connection duration 100ms, got %v", r.ConnectionDuration())
	}

	if r.RequestDuration() != 50*time.Millisecond {
		t.Errorf("Expected request duration 50ms, got %v", r.RequestDuration())
	}

	if r.TotalDuration() != 200*time.Millisecond {
		t.Errorf("Expected total duration 200ms, got %v", r.TotalDuration())
	}

	if r.TTFB() != 150*time.Millisecond {
		t.Errorf("Expected TTFB 150ms, got %v", r.TTFB())
	}
}

// TestResultJSON verifies JSON serialization.
func TestResultJSON(t *testing.T) {
	now := time.Now()
	r := &Result{
		Start:       now,
		ConnectDone: now.Add(100 * time.Millisecond),
		FirstByte:   now.Add(150 * time.Millisecond),
		End:         now.Add(200 * time.Millisecond),
	}

	data, err := r.MarshalJSON()
	if err != nil {
		t.Fatalf("Failed to marshal JSON: %v", err)
	}

	expected := `{"connect_ms":100,"ttfb_ms":150,"total_ms":200,"request_ms":50}`
	if string(data) != expected {
		t.Errorf("Expected JSON %s, got %s", expected, string(data))
	}
}

// TestResultWithError verifies error handling.
func TestResultWithError(t *testing.T) {
	r := &Result{
		Error: fmt.Errorf("connection refused"),
	}

	data, err := r.MarshalJSON()
	if err != nil {
		t.Fatalf("Failed to marshal JSON: %v", err)
	}

	if string(data) != `{"connect_ms":0,"ttfb_ms":0,"total_ms":0,"request_ms":0,"error":"connection refused"}` {
		t.Errorf("Unexpected JSON for error result: %s", string(data))
	}
}

// TestLegacyMethods verifies deprecated methods return 0.
func TestLegacyMethods(t *testing.T) {
	f := New()

	if f.ConnDuration() != 0 {
		t.Error("Expected ConnDuration to return 0")
	}

	if f.ReqDuration() != 0 {
		t.Error("Expected ReqDuration to return 0")
	}

	if f.Duration() != 0 {
		t.Error("Expected Duration to return 0")
	}
}

// TestHTTPTraceIntegration verifies httptrace timing capture.
func TestHTTPTraceIntegration(t *testing.T) {
	// Create a test server that supports HTTPS
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(5 * time.Millisecond) // Simulate processing
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Hello, World!"))
	}))
	defer server.Close()

	// Get the test server's HTTP client (which trusts the test certificate)
	// and wrap its transport with Ferret
	client := server.Client()
	ferret := New(WithTransport(client.Transport))
	client.Transport = ferret

	// Make request
	req, err := http.NewRequest("GET", server.URL, nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	// Get result
	result := GetResult(resp.Request)
	if result == nil {
		t.Fatal("No result found")
	}

	// Verify all timing fields are populated
	if result.Start.IsZero() {
		t.Error("Start time not set")
	}
	if result.ConnectStart.IsZero() {
		t.Error("ConnectStart time not set")
	}
	if result.ConnectDone.IsZero() {
		t.Error("ConnectDone time not set")
	}
	if result.TLSHandshakeStart.IsZero() {
		t.Error("TLSHandshakeStart time not set")
	}
	if result.TLSHandshakeDone.IsZero() {
		t.Error("TLSHandshakeDone time not set")
	}
	if result.FirstByte.IsZero() {
		t.Error("FirstByte time not set")
	}
	if result.End.IsZero() {
		t.Error("End time not set")
	}

	// Verify basic timing order (some events may happen simultaneously)
	if !result.Start.Before(result.End) {
		t.Error("Start should be before End")
	}
	if !result.ConnectStart.Before(result.FirstByte) {
		t.Error("ConnectStart should be before FirstByte")
	}
	if !result.TLSHandshakeStart.Before(result.FirstByte) {
		t.Error("TLSHandshakeStart should be before FirstByte")
	}
	// Note: ConnectDone and TLSHandshakeDone may be reported at the same time
	// or in slightly different order depending on the implementation

	// Verify durations
	if result.ConnectionDuration() <= 0 {
		t.Error("ConnectionDuration should be positive")
	}
	if result.TLSDuration() <= 0 {
		t.Error("TLSDuration should be positive")
	}
	if result.ServerProcessingDuration() <= 0 {
		t.Error("ServerProcessingDuration should be positive")
	}
	if result.TTFB() <= 0 {
		t.Error("TTFB should be positive")
	}
	if result.TotalDuration() <= 0 {
		t.Error("TotalDuration should be positive")
	}
}

// TestHTTPTraceWithPlainHTTP verifies httptrace works without TLS.
func TestHTTPTraceWithPlainHTTP(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	defer server.Close()

	ferret := New()
	client := &http.Client{Transport: ferret}

	req, err := http.NewRequest("GET", server.URL, nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	result := GetResult(resp.Request)
	if result == nil {
		t.Fatal("No result found")
	}

	// TLS fields should be zero for plain HTTP
	if !result.TLSHandshakeStart.IsZero() {
		t.Error("TLSHandshakeStart should be zero for plain HTTP")
	}
	if !result.TLSHandshakeDone.IsZero() {
		t.Error("TLSHandshakeDone should be zero for plain HTTP")
	}
	if result.TLSDuration() != 0 {
		t.Error("TLSDuration should be 0 for plain HTTP")
	}

	// Other timings should still be present
	if result.ConnectStart.IsZero() {
		t.Error("ConnectStart should be set")
	}
	if result.ConnectDone.IsZero() {
		t.Error("ConnectDone should be set")
	}
}

// TestResultPhaseDurations verifies the new phase duration methods.
func TestResultPhaseDurations(t *testing.T) {
	now := time.Now()
	r := &Result{
		Start:             now,
		DNSStart:          now.Add(10 * time.Millisecond),
		DNSDone:           now.Add(20 * time.Millisecond),
		ConnectStart:      now.Add(20 * time.Millisecond),
		TLSHandshakeStart: now.Add(30 * time.Millisecond),
		TLSHandshakeDone:  now.Add(50 * time.Millisecond),
		ConnectDone:       now.Add(50 * time.Millisecond),
		FirstByte:         now.Add(100 * time.Millisecond),
		End:               now.Add(150 * time.Millisecond),
	}

	// Test phase durations
	if r.DNSDuration() != 10*time.Millisecond {
		t.Errorf("Expected DNS duration 10ms, got %v", r.DNSDuration())
	}

	if r.ConnectionDuration() != 30*time.Millisecond {
		t.Errorf("Expected connection duration 30ms, got %v", r.ConnectionDuration())
	}

	if r.TLSDuration() != 20*time.Millisecond {
		t.Errorf("Expected TLS duration 20ms, got %v", r.TLSDuration())
	}

	if r.ServerProcessingDuration() != 50*time.Millisecond {
		t.Errorf("Expected server processing duration 50ms, got %v", r.ServerProcessingDuration())
	}

	if r.DataTransferDuration() != 50*time.Millisecond {
		t.Errorf("Expected data transfer duration 50ms, got %v", r.DataTransferDuration())
	}

	if r.TTFB() != 100*time.Millisecond {
		t.Errorf("Expected TTFB 100ms, got %v", r.TTFB())
	}

	if r.TotalDuration() != 150*time.Millisecond {
		t.Errorf("Expected total duration 150ms, got %v", r.TotalDuration())
	}
}

// TestResultStringWithDetailedTimings verifies String output includes all timings.
func TestResultStringWithDetailedTimings(t *testing.T) {
	now := time.Now()
	r := &Result{
		Start:             now,
		DNSStart:          now.Add(10 * time.Millisecond),
		DNSDone:           now.Add(20 * time.Millisecond),
		ConnectStart:      now.Add(20 * time.Millisecond),
		TLSHandshakeStart: now.Add(30 * time.Millisecond),
		TLSHandshakeDone:  now.Add(50 * time.Millisecond),
		ConnectDone:       now.Add(50 * time.Millisecond),
		FirstByte:         now.Add(100 * time.Millisecond),
		End:               now.Add(150 * time.Millisecond),
	}

	str := r.String()

	// Check that all timing components are present
	if !contains(str, "total=150ms") {
		t.Errorf("String should contain total time: %s", str)
	}
	if !contains(str, "dns=10ms") {
		t.Errorf("String should contain DNS time: %s", str)
	}
	if !contains(str, "connect=30ms") {
		t.Errorf("String should contain connect time: %s", str)
	}
	if !contains(str, "tls=20ms") {
		t.Errorf("String should contain TLS time: %s", str)
	}
	if !contains(str, "ttfb=100ms") {
		t.Errorf("String should contain TTFB: %s", str)
	}
}

func contains(s, substr string) bool {
	if len(s) < len(substr) {
		return false
	}
	if s == substr {
		return true
	}
	if len(s) > len(substr) {
		if s[:len(substr)] == substr || s[len(s)-len(substr):] == substr {
			return true
		}
		if len(substr) > 0 {
			return findSubstring(s, substr)
		}
	}
	return false
}

func findSubstring(s, substr string) bool {
	for i := 1; i < len(s)-len(substr)+1; i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
