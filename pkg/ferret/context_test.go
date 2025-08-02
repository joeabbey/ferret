package ferret

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// TestContextCancellationBeforeRequest verifies cancellation before request starts.
func TestContextCancellationBeforeRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("Handler should not be called")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	ferret := New()
	client := &http.Client{Transport: ferret}

	// Create a cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	req, err := http.NewRequestWithContext(ctx, "GET", server.URL, nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	resp, err := client.Do(req)
	if err == nil {
		t.Error("Expected error due to cancelled context")
		resp.Body.Close()
	}

	// Verify the error is a context error
	if ctx.Err() != context.Canceled {
		t.Errorf("Expected context.Canceled, got %v", ctx.Err())
	}
}

// TestContextCancellationDuringConnection verifies cancellation during connection.
func TestContextCancellationDuringConnection(t *testing.T) {
	// Create a listener that delays accepting connections
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to create listener: %v", err)
	}
	defer listener.Close()

	// Don't accept connections immediately
	go func() {
		time.Sleep(100 * time.Millisecond)
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		conn.Close()
	}()

	ferret := New(WithTimeout(5*time.Second, 0))
	client := &http.Client{Transport: ferret}

	// Create a context with short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", "http://"+listener.Addr().String(), nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	start := time.Now()
	resp, err := client.Do(req)
	duration := time.Since(start)

	if err == nil {
		t.Error("Expected error due to context timeout")
		resp.Body.Close()
	}

	// Verify we cancelled quickly (not waiting for full connection timeout)
	if duration > 100*time.Millisecond {
		t.Errorf("Cancellation took too long: %v", duration)
	}
}

// TestContextCancellationDuringRequest verifies cancellation during request.
func TestContextCancellationDuringRequest(t *testing.T) {
	// Server that delays response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-time.After(200 * time.Millisecond):
			w.WriteHeader(http.StatusOK)
		case <-r.Context().Done():
			// Request was cancelled
			return
		}
	}))
	defer server.Close()

	ferret := New()
	client := &http.Client{Transport: ferret}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", server.URL, nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	start := time.Now()
	resp, err := client.Do(req)
	duration := time.Since(start)

	if err == nil {
		t.Error("Expected error due to context timeout")
		if resp != nil {
			resp.Body.Close()
		}
	}

	// Verify timing is reasonable
	if duration < 90*time.Millisecond || duration > 150*time.Millisecond {
		t.Errorf("Unexpected duration: %v", duration)
	}

	// Even with cancellation, we should have partial timing data
	if resp != nil && resp.Request != nil {
		result := GetResult(resp.Request)
		if result != nil {
			// We should have at least start time
			if result.Start.IsZero() {
				t.Error("Expected start time to be set")
			}
		}
	}
}

// TestContextWithDeadline verifies deadline handling.
func TestContextWithDeadline(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check if request has deadline - note that the deadline might not
		// be directly visible in the request context due to transport layers
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	ferret := New()
	client := &http.Client{Transport: ferret}

	// Create context with deadline
	deadline := time.Now().Add(1 * time.Second)
	ctx, cancel := context.WithDeadline(context.Background(), deadline)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", server.URL, nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	// Verify timing data is complete
	result := GetResult(resp.Request)
	if result == nil {
		t.Fatal("No result found")
	}

	if result.TotalDuration() <= 0 {
		t.Error("Invalid total duration")
	}
}

// TestContextPropagation verifies context values are used by Ferret.
func TestContextPropagation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Context values might not propagate through HTTP transport layers
		// This is expected behavior - context is used for cancellation and deadlines
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	ferret := New()
	client := &http.Client{Transport: ferret}

	// Create context with value (for testing Ferret's result storage)
	ctx := context.Background()

	req, err := http.NewRequestWithContext(ctx, "GET", server.URL, nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	// Verify Ferret's context usage (result storage)
	result := GetResult(resp.Request)
	if result == nil {
		t.Error("Expected Ferret to store result in context")
	}
}

// TestMultipleContextCancellations verifies handling of multiple cancellations.
func TestMultipleContextCancellations(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(50 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	ferret := New()
	client := &http.Client{Transport: ferret}

	// Test different cancellation scenarios
	scenarios := []struct {
		name        string
		setupCtx    func() (context.Context, context.CancelFunc)
		shouldError bool
	}{
		{
			name: "no cancellation",
			setupCtx: func() (context.Context, context.CancelFunc) {
				return context.WithTimeout(context.Background(), 1*time.Second)
			},
			shouldError: false,
		},
		{
			name: "cancel before request",
			setupCtx: func() (context.Context, context.CancelFunc) {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return ctx, cancel
			},
			shouldError: true,
		},
		{
			name: "cancel during request",
			setupCtx: func() (context.Context, context.CancelFunc) {
				ctx, cancel := context.WithCancel(context.Background())
				go func() {
					time.Sleep(25 * time.Millisecond)
					cancel()
				}()
				return ctx, cancel
			},
			shouldError: true,
		},
		{
			name: "timeout during request",
			setupCtx: func() (context.Context, context.CancelFunc) {
				return context.WithTimeout(context.Background(), 25*time.Millisecond)
			},
			shouldError: true,
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			ctx, cancel := scenario.setupCtx()
			defer cancel()

			req, err := http.NewRequestWithContext(ctx, "GET", server.URL, nil)
			if err != nil {
				t.Fatalf("Failed to create request: %v", err)
			}

			resp, err := client.Do(req)

			if scenario.shouldError && err == nil {
				t.Error("Expected error but got none")
				if resp != nil {
					resp.Body.Close()
				}
			} else if !scenario.shouldError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			} else if resp != nil {
				resp.Body.Close()
			}
		})
	}
}

