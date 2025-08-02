package ferret_test

import (
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"sync"
	"time"

	"github.com/joeabbey/ferret/pkg/ferret"
	"github.com/prometheus/client_golang/prometheus"
)

// ExampleNew demonstrates basic usage of Ferret.
func ExampleNew() {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Hello, World!"))
	}))
	defer server.Close()

	// Create a Ferret transport
	f := ferret.New()
	client := &http.Client{Transport: f}

	// Make a request
	req, _ := http.NewRequest("GET", server.URL, nil)
	resp, err := client.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	// Get timing information
	result := ferret.GetResult(resp.Request)
	if result != nil {
		fmt.Printf("Total request time: %v\n", result.TotalDuration())
		fmt.Printf("Connection time: %v\n", result.ConnectionDuration())
	}
}

// ExampleNew_withOptions demonstrates using Ferret with options.
func ExampleNew_withOptions() {
	// Create Ferret with custom configuration
	f := ferret.New(
		ferret.WithKeepAlives(false),        // Disable keep-alives for cleaner measurements
		ferret.WithTimeout(5*time.Second, 10*time.Second), // 5s connect, 10s total timeout
		ferret.WithTLSHandshakeTimeout(3*time.Second),
	)

	client := &http.Client{Transport: f}

	// Use the client for requests
	resp, err := client.Get("https://example.com")
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	fmt.Printf("Status: %s\n", resp.Status)
}

// ExampleGetResult demonstrates retrieving timing information.
func ExampleGetResult() {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate some processing time
		time.Sleep(50 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	f := ferret.New()
	client := &http.Client{Transport: f}

	req, _ := http.NewRequest("GET", server.URL, nil)
	resp, err := client.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	// Get detailed timing information
	result := ferret.GetResult(resp.Request)
	if result != nil {
		fmt.Printf("Request phases:\n")
		fmt.Printf("  DNS lookup: %v\n", result.DNSDuration())
		fmt.Printf("  TCP connection: %v\n", result.ConnectionDuration())
		fmt.Printf("  TLS handshake: %v\n", result.TLSDuration())
		fmt.Printf("  Time to first byte: %v\n", result.TTFB())
		fmt.Printf("  Total time: %v\n", result.TotalDuration())
	}
}


// ExampleWithPrometheus demonstrates Prometheus integration.
func ExampleWithPrometheus() {
	// Create Prometheus histogram
	histogram := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "http_request_duration_seconds",
			Help: "HTTP request duration by phase",
		},
		[]string{"phase", "method", "host", "code", "status"},
	)

	// Register the histogram
	prometheus.MustRegister(histogram)

	// Create Ferret with Prometheus instrumentation
	config := ferret.PrometheusConfig{
		DurationHistogram: histogram,
		DetailedMetrics:   true,
	}

	f := ferret.New(ferret.WithPrometheus(config))
	client := &http.Client{Transport: f}

	// Make requests - metrics will be automatically collected
	resp, err := client.Get("https://example.com")
	if err == nil {
		resp.Body.Close()
		fmt.Println("Request completed, metrics collected")
	}
}

// ExampleNew_concurrentRequests demonstrates concurrent usage.
func ExampleNew_concurrentRequests() {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Create a single Ferret instance - it's safe for concurrent use
	f := ferret.New()
	client := &http.Client{Transport: f}

	// Make concurrent requests
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			req, _ := http.NewRequest("GET", server.URL, nil)
			resp, err := client.Do(req)
			if err != nil {
				log.Printf("Request %d failed: %v", id, err)
				return
			}
			defer resp.Body.Close()

			result := ferret.GetResult(resp.Request)
			if result != nil {
				fmt.Printf("Request %d completed in %v\n", id, result.TotalDuration())
			}
		}(i)
	}

	wg.Wait()
}