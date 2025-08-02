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