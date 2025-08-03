package ferret

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestHighConcurrency verifies Ferret handles high concurrent load.
func TestHighConcurrency(t *testing.T) {
	// Create a test server that tracks concurrent requests
	var activeRequests int32
	var maxConcurrent int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		// Track concurrent requests
		current := atomic.AddInt32(&activeRequests, 1)
		defer atomic.AddInt32(&activeRequests, -1)

		// Update max if needed
		for {
			maxVal := atomic.LoadInt32(&maxConcurrent)
			if current <= maxVal || atomic.CompareAndSwapInt32(&maxConcurrent, maxVal, current) {
				break
			}
		}

		// Simulate some work
		time.Sleep(10 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	}))
	defer server.Close()

	// Create Ferret transport
	ferret := New()
	client := &http.Client{Transport: ferret}

	// Number of concurrent requests
	concurrency := 100
	iterations := 10

	var wg sync.WaitGroup
	errors := make(chan error, concurrency*iterations)

	// Launch concurrent requests
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(_ int) {
			defer wg.Done()

			for j := 0; j < iterations; j++ {
				req, err := http.NewRequest("GET", server.URL, nil)
				if err != nil {
					errors <- err
					continue
				}

				resp, err := client.Do(req)
				if err != nil {
					errors <- err
					continue
				}

				// Verify we got a result
				result := GetResult(resp.Request)
				if result == nil {
					errors <- err
					continue
				}

				// Verify timing data
				if result.TotalDuration() <= 0 {
					errors <- err
				}

				_ = resp.Body.Close()
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	// Check for errors
	errorCount := 0
	for err := range errors {
		if err != nil {
			errorCount++
			if errorCount <= 5 { // Only log first 5 errors
				t.Errorf("Request failed: %v", err)
			}
		}
	}

	if errorCount > 0 {
		t.Errorf("Total %d requests failed out of %d", errorCount, concurrency*iterations)
	}

	t.Logf("Max concurrent requests handled: %d", atomic.LoadInt32(&maxConcurrent))
}

// TestConcurrentWithMultipleTransports verifies multiple Ferret instances work correctly.
func TestConcurrentWithMultipleTransports(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Transport-ID", r.Header.Get("X-Transport-ID"))
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Create multiple Ferret transports
	transports := make([]*Ferret, 5)
	clients := make([]*http.Client, 5)
	for i := range transports {
		transports[i] = New()
		clients[i] = &http.Client{Transport: transports[i]}
	}

	var wg sync.WaitGroup
	iterations := 20

	// Use each transport concurrently
	for i, client := range clients {
		for j := 0; j < iterations; j++ {
			wg.Add(1)
			go func(transportID int, _ int) {
				defer wg.Done()

				req, err := http.NewRequest("GET", server.URL, nil)
				if err != nil {
					t.Errorf("Failed to create request: %v", err)
					return
				}
				req.Header.Set("X-Transport-ID", fmt.Sprintf("%d", transportID))

				resp, err := client.Do(req)
				if err != nil {
					t.Errorf("Request failed: %v", err)
					return
				}
				defer func() { _ = resp.Body.Close() }()

				// Verify we used the right transport
				if resp.Header.Get("X-Transport-ID") != fmt.Sprintf("%d", transportID) {
					t.Errorf("Wrong transport ID in response")
				}

				// Verify timing data
				result := GetResult(resp.Request)
				if result == nil {
					t.Error("No result found")
					return
				}

				if result.TotalDuration() <= 0 {
					t.Error("Invalid duration")
				}
			}(i, j)
		}
	}

	wg.Wait()
}

// TestRaceConditionWithOptions verifies option application is thread-safe.
func TestRaceConditionWithOptions(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	var wg sync.WaitGroup

	// Concurrently create Ferret instances with different options
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			var ferret *Ferret

			// Use different options based on ID
			switch id % 3 {
			case 0:
				ferret = New(WithKeepAlives(true))
			case 1:
				ferret = New(WithKeepAlives(false), WithTimeout(5*time.Second, 10*time.Second))
			case 2:
				ferret = New(WithTLSHandshakeTimeout(3 * time.Second))
			}

			client := &http.Client{Transport: ferret}

			req, _ := http.NewRequest("GET", server.URL, nil)
			resp, err := client.Do(req)
			if err != nil {
				t.Errorf("Request failed: %v", err)
				return
			}
			_ = resp.Body.Close()
		}(i)
	}

	wg.Wait()
}

// TestConcurrentMetricsCollection verifies metrics are collected correctly under load.
func TestConcurrentMetricsCollection(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Variable response times
		if r.URL.Query().Get("delay") == "true" {
			time.Sleep(50 * time.Millisecond)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	ferret := New()
	client := &http.Client{Transport: ferret}

	var wg sync.WaitGroup
	results := make([]*Result, 100)

	// Make concurrent requests with different delays
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()

			url := server.URL
			if index%2 == 0 {
				url += "?delay=true"
			}

			req, _ := http.NewRequest("GET", url, nil)
			resp, err := client.Do(req)
			if err != nil {
				t.Errorf("Request %d failed: %v", index, err)
				return
			}
			defer func() { _ = resp.Body.Close() }()

			results[index] = GetResult(resp.Request)
		}(i)
	}

	wg.Wait()

	// Verify all results are unique and have valid data
	slowCount := 0
	fastCount := 0

	for i, result := range results {
		if result == nil {
			t.Errorf("Result %d is nil", i)
			continue
		}

		duration := result.TotalDuration()
		if duration <= 0 {
			t.Errorf("Result %d has invalid duration", i)
			continue
		}

		// Check if delays were applied correctly
		if i%2 == 0 && duration > 40*time.Millisecond {
			slowCount++
		} else if i%2 == 1 && duration < 40*time.Millisecond {
			fastCount++
		}
	}

	// Verify we got roughly the expected distribution
	if slowCount < 40 || fastCount < 40 {
		t.Errorf("Unexpected distribution: slow=%d, fast=%d", slowCount, fastCount)
	}
}

