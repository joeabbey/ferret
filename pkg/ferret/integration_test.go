package ferret

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
	"runtime"
)

// TestIntegrationWithRealServer tests against a real HTTP server.
func TestIntegrationWithRealServer(t *testing.T) {
	// Create a real TLS server
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate different response times based on path
		switch r.URL.Path {
		case "/slow":
			time.Sleep(100 * time.Millisecond)
		case "/fast":
			// No delay
		case "/error":
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("OK"))
	}))
	defer server.Close()

	// Create Ferret with the test server's client (trusts the test cert)
	client := server.Client()
	ferret := New(WithTransport(client.Transport))
	client.Transport = ferret

	tests := []struct {
		path         string
		minDuration  time.Duration
		expectError  bool
		expectStatus int
	}{
		{"/fast", 0, false, http.StatusOK},
		{"/slow", 90 * time.Millisecond, false, http.StatusOK},
		{"/error", 0, false, http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			req, err := http.NewRequest("GET", server.URL+tt.path, nil)
			if err != nil {
				t.Fatalf("Failed to create request: %v", err)
			}

			start := time.Now()
			resp, err := client.Do(req)
			elapsed := time.Since(start)

			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			} else if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if resp != nil {
				defer func() { _ = resp.Body.Close() }()

				if resp.StatusCode != tt.expectStatus {
					t.Errorf("Expected status %d, got %d", tt.expectStatus, resp.StatusCode)
				}

				// Verify timing
				result := GetResult(resp.Request)
				if result == nil {
					t.Fatal("No timing result found")
				}

				if elapsed < tt.minDuration {
					t.Errorf("Request completed too quickly: %v < %v", elapsed, tt.minDuration)
				}

				// Verify all timing fields for HTTPS
				if !result.DNSStart.IsZero() && !result.DNSDone.IsZero() {
					if result.DNSDuration() <= 0 {
						t.Error("Expected positive DNS duration")
					}
				}

				if result.TLSDuration() <= 0 {
					t.Error("Expected positive TLS duration for HTTPS")
				}

				// On Windows, timing might be zero due to clock resolution
				connDur := result.ConnectionDuration()
				if connDur < 0 {
					t.Error("Connection duration should not be negative")
				} else if connDur == 0 && runtime.GOOS != "windows" {
					t.Error("Expected positive connection duration")
				}

				if result.TTFB() <= 0 {
					t.Error("Expected positive TTFB")
				}
			}
		})
	}
}

// TestIntegrationConnectionFailures tests various connection failure scenarios.
func TestIntegrationConnectionFailures(t *testing.T) {
	ferret := New(WithTimeout(1*time.Second, 2*time.Second))
	client := &http.Client{Transport: ferret}

	tests := []struct {
		name        string
		url         string
		expectError bool
		errorType   string
	}{
		{
			name:        "non-existent host",
			url:         "http://non-existent-host-12345.test",
			expectError: true,
			errorType:   "lookup",
		},
		{
			name:        "connection refused",
			url:         "http://127.0.0.1:1", // Port 1 is typically refused
			expectError: true,
			errorType:   "refused",
		},
		{
			name:        "invalid URL",
			url:         "http://[invalid",
			expectError: true,
			errorType:   "parse",
		},
		{
			name:        "timeout",
			url:         "http://192.0.2.1:80", // TEST-NET-1, won't route
			expectError: true,
			errorType:   "timeout",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := http.NewRequest("GET", tt.url, nil)
			if err != nil {
				if tt.errorType == "parse" {
					// Expected parse error
					return
				}
				t.Fatalf("Failed to create request: %v", err)
			}

			resp, err := client.Do(req)
			if err == nil {
				t.Error("Expected error but got none")
				if resp != nil {
					_ = resp.Body.Close()
				}
				return
			}

			// Verify we still get timing data even on error
			if req != nil {
				result := GetResult(req)
				if result != nil {
					// We should at least have start time
					if result.Start.IsZero() {
						t.Error("Expected start time even on error")
					}
				}
			}
		})
	}
}

// TestIntegrationWithProxy tests proxy support.
func TestIntegrationWithProxy(t *testing.T) {
	// Create a simple proxy server
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "CONNECT" {
			// For non-CONNECT requests, we're acting as an HTTP proxy
			w.Header().Set("X-Proxy", "true")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("Proxied"))
			return
		}
		// For CONNECT, we'd need to handle tunneling (skip for this test)
		w.WriteHeader(http.StatusMethodNotAllowed)
	}))
	defer proxy.Close()

	// Create transport with proxy
	proxyURL, _ := url.Parse(proxy.URL)
	baseTransport := &http.Transport{
		Proxy: http.ProxyURL(proxyURL),
	}

	ferret := New(WithTransport(baseTransport))
	client := &http.Client{Transport: ferret}

	// Make request through proxy
	req, err := http.NewRequest("GET", "http://example.com", nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Verify proxy was used
	if resp.Header.Get("X-Proxy") != "true" {
		t.Error("Request didn't go through proxy")
	}

	// Verify timing data
	result := GetResult(resp.Request)
	if result == nil {
		t.Fatal("No timing result found")
	}

	totalDur := result.TotalDuration()
	if totalDur < 0 {
		t.Error("Total duration should not be negative")
	} else if totalDur == 0 && runtime.GOOS != "windows" {
		t.Error("Expected positive total duration")
	}
}

// TestIntegrationHTTP2 tests HTTP/2 support.
func TestIntegrationHTTP2(t *testing.T) {
	// Create an HTTP/2 server
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Protocol", r.Proto)
		w.WriteHeader(http.StatusOK)
	}))

	// Enable HTTP/2
	server.TLS = &tls.Config{
		MinVersion: tls.VersionTLS12,
		NextProtos: []string{"h2", "http/1.1"},
	}
	server.StartTLS()
	defer server.Close()

	// Create client with HTTP/2 support
	client := server.Client()
	baseTransport := client.Transport.(*http.Transport)
	baseTransport.ForceAttemptHTTP2 = true

	ferret := New(WithTransport(baseTransport))
	client.Transport = ferret

	req, err := http.NewRequest("GET", server.URL, nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Verify protocol
	if proto := resp.Header.Get("X-Protocol"); proto != "HTTP/2.0" {
		t.Logf("Note: Expected HTTP/2.0, got %s (HTTP/2 may not be available in test env)", proto)
	}

	// Verify timing still works with HTTP/2
	result := GetResult(resp.Request)
	if result == nil {
		t.Fatal("No timing result found")
	}

	totalDur := result.TotalDuration()
	if totalDur < 0 {
		t.Error("Total duration should not be negative")
	} else if totalDur == 0 && runtime.GOOS != "windows" {
		t.Error("Expected positive total duration")
	}
}

// TestIntegrationLargeResponse tests handling of large responses.
func TestIntegrationLargeResponse(t *testing.T) {
	// Create server that sends large response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		// Send 1MB of data
		data := make([]byte, 1024*1024)
		for i := range data {
			data[i] = byte(i % 256)
		}
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(data)))
		_, _ = w.Write(data)
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
	defer func() { _ = resp.Body.Close() }()

	// Read the entire response
	buf := make([]byte, 1024)
	total := 0
	for {
		n, err := resp.Body.Read(buf)
		total += n
		if err != nil {
			break
		}
	}

	if total != 1024*1024 {
		t.Errorf("Expected 1MB response, got %d bytes", total)
	}

	// Verify timing
	result := GetResult(resp.Request)
	if result == nil {
		t.Fatal("No timing result found")
	}

	// Data transfer should take some time
	// On Windows, timing might be zero due to clock resolution
	transfer := result.DataTransferDuration()
	if transfer < 0 {
		t.Error("Data transfer duration should not be negative")
	} else if transfer == 0 && runtime.GOOS != "windows" {
		t.Error("Expected positive data transfer duration for large response")
	}

	// TTFB should be less than total duration
	// On Windows, due to timing resolution, TTFB might equal TotalDuration for fast operations
	ttfb := result.TTFB()
	totalDuration := result.TotalDuration()
	if ttfb > 0 && totalDuration > 0 {
		if ttfb > totalDuration {
			t.Error("TTFB should not be greater than total duration")
		} else if ttfb == totalDuration && runtime.GOOS != "windows" {
			t.Error("TTFB should be less than total duration")
		}
	}
}

// TestIntegrationWithRedirects tests redirect handling.
func TestIntegrationWithRedirects(t *testing.T) {
	redirectCount := 0

	// Create server with redirects
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			http.Redirect(w, r, "/redirect1", http.StatusFound)
		case "/redirect1":
			redirectCount++
			http.Redirect(w, r, "/redirect2", http.StatusFound)
		case "/redirect2":
			redirectCount++
			http.Redirect(w, r, "/final", http.StatusFound)
		case "/final":
			redirectCount++
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("Final"))
		}
	}))
	defer server.Close()

	ferret := New()

	// Use custom client that follows redirects
	client := &http.Client{
		Transport: ferret,
		// Default redirect behavior - follow up to 10 redirects
	}

	req, err := http.NewRequest("GET", server.URL, nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Should have followed redirects
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected 200 OK after redirects, got %d", resp.StatusCode)
	}

	// Each redirect is a separate request, so we only get timing for the final one
	result := GetResult(resp.Request)
	if result == nil {
		t.Fatal("No timing result found")
	}

	totalDur := result.TotalDuration()
	if totalDur < 0 {
		t.Error("Total duration should not be negative")
	} else if totalDur == 0 && runtime.GOOS != "windows" {
		t.Error("Expected positive total duration")
	}
}

