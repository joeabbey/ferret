// Package main demonstrates Ferret integration with Prometheus metrics.
package main

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/joeabbey/ferret/pkg/ferret"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	// Create Prometheus metrics
	httpDuration := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "http_client_request_duration_seconds",
			Help: "Duration of HTTP client requests by phase",
			Buckets: []float64{
				0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5, 5.0,
			},
		},
		[]string{"phase", "method", "host", "code", "status"},
	)

	httpRequests := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_client_requests_total",
			Help: "Total number of HTTP client requests",
		},
		[]string{"method", "host", "code", "status"},
	)

	httpInFlight := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "http_client_in_flight_requests",
		Help: "Number of HTTP client requests currently in flight",
	})

	// Register metrics
	prometheus.MustRegister(httpDuration, httpRequests, httpInFlight)

	// Create Ferret with Prometheus instrumentation
	prometheusConfig := ferret.PrometheusConfig{
		DurationHistogram: httpDuration,
		RequestCounter:    httpRequests,
		InFlightGauge:     httpInFlight,
		DetailedMetrics:   true, // Enable per-phase metrics
	}

	f := ferret.New(ferret.WithPrometheus(prometheusConfig))
	client := &http.Client{Transport: f}

	// Start Prometheus metrics endpoint
	go func() {
		mux := http.NewServeMux()
		mux.Handle("/metrics", promhttp.Handler())
		
		server := &http.Server{
			Addr:         ":9090",
			Handler:      mux,
			ReadTimeout:  10 * time.Second,
			WriteTimeout: 10 * time.Second,
			IdleTimeout:  120 * time.Second,
		}
		
		log.Println("Metrics available at http://localhost:9090/metrics")
		log.Fatal(server.ListenAndServe())
	}()

	// Make some example requests
	urls := []string{
		"https://example.com",
		"https://golang.org",
		"https://github.com",
		"https://httpstat.us/200?sleep=100",  // Delayed response
		"https://httpstat.us/500",              // Error response
	}

	fmt.Println("Making HTTP requests...")
	for _, url := range urls {
		fmt.Printf("Requesting %s... ", url)
		
		resp, err := client.Get(url)
		if err != nil {
			fmt.Printf("ERROR: %v\n", err)
			continue
		}
		
		resp.Body.Close()
		fmt.Printf("Status: %d\n", resp.StatusCode)
		
		// Small delay between requests
		time.Sleep(100 * time.Millisecond)
	}

	fmt.Println("\nMetrics have been collected. Check http://localhost:9090/metrics")
	fmt.Println("Some example metrics queries:")
	fmt.Println("  - http_client_requests_total")
	fmt.Println("  - http_client_request_duration_seconds{phase=\"total\"}")
	fmt.Println("  - http_client_request_duration_seconds{phase=\"dns\"}")
	fmt.Println("  - http_client_request_duration_seconds{phase=\"tls\"}")
	fmt.Println("  - http_client_in_flight_requests")
	
	// Keep the program running
	select {}
}

// Example Prometheus queries:
//
// Average request duration by phase:
// rate(http_client_request_duration_seconds_sum[5m]) / rate(http_client_request_duration_seconds_count[5m])
//
// Request rate by status:
// sum(rate(http_client_requests_total[5m])) by (status)
//
// 95th percentile latency by host:
// histogram_quantile(0.95, sum(rate(http_client_request_duration_seconds_bucket{phase="total"}[5m])) by (host, le))
//
// DNS resolution time by host:
// http_client_request_duration_seconds{phase="dns",quantile="0.5"}