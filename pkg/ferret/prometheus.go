package ferret

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// PrometheusConfig holds configuration for Prometheus metrics collection.
type PrometheusConfig struct {
	// Histogram for tracking phase durations
	DurationHistogram *prometheus.HistogramVec
	
	// Optional: Counter for total requests
	RequestCounter *prometheus.CounterVec
	
	// Optional: Gauge for in-flight requests
	InFlightGauge prometheus.Gauge
	
	// Whether to include detailed phase metrics
	DetailedMetrics bool
}

// WithPrometheus returns an option that enables Prometheus metrics collection.
func WithPrometheus(config PrometheusConfig) Option {
	return func(f *Ferret) {
		// Wrap the existing transport with Prometheus instrumentation
		f.next = &prometheusTransport{
			next:   f.next,
			config: config,
			ferret: f,
		}
	}
}

// prometheusTransport wraps a RoundTripper to collect Prometheus metrics.
type prometheusTransport struct {
	next   http.RoundTripper
	config PrometheusConfig
	ferret *Ferret
}

// RoundTrip implements http.RoundTripper with Prometheus metrics collection.
func (t *prometheusTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Track in-flight requests if gauge is provided
	if t.config.InFlightGauge != nil {
		t.config.InFlightGauge.Inc()
		defer t.config.InFlightGauge.Dec()
	}

	// Execute the request through the Ferret transport
	resp, err := t.next.RoundTrip(req)

	// Get the result from the request
	result := GetResult(req)
	if result == nil && resp != nil && resp.Request != nil {
		result = GetResult(resp.Request)
	}
	
	if result != nil {
		// Extract labels
		method := req.Method
		host := req.URL.Host
		code := "0"
		if resp != nil {
			code = strconv.Itoa(resp.StatusCode)
		}
		status := "success"
		if err != nil {
			status = "error"
		}

		// Common labels for all metrics
		labels := prometheus.Labels{
			"method": method,
			"host":   host,
			"code":   code,
			"status": status,
		}

		// Record request count
		if t.config.RequestCounter != nil {
			t.config.RequestCounter.With(labels).Inc()
		}

		// Record duration histogram
		if t.config.DurationHistogram != nil {
			// Total duration
			t.config.DurationHistogram.With(prometheus.Labels{
				"phase":  "total",
				"method": method,
				"host":   host,
				"code":   code,
				"status": status,
			}).Observe(result.TotalDuration().Seconds())

			// Record detailed phase metrics if enabled
			if t.config.DetailedMetrics {
				// DNS duration
				if dns := result.DNSDuration(); dns > 0 {
					t.config.DurationHistogram.With(prometheus.Labels{
						"phase":  "dns",
						"method": method,
						"host":   host,
						"code":   code,
						"status": status,
					}).Observe(dns.Seconds())
				}

				// Connection duration
				if conn := result.ConnectionDuration(); conn > 0 {
					t.config.DurationHistogram.With(prometheus.Labels{
						"phase":  "connect",
						"method": method,
						"host":   host,
						"code":   code,
						"status": status,
					}).Observe(conn.Seconds())
				}

				// TLS duration
				if tls := result.TLSDuration(); tls > 0 {
					t.config.DurationHistogram.With(prometheus.Labels{
						"phase":  "tls",
						"method": method,
						"host":   host,
						"code":   code,
						"status": status,
					}).Observe(tls.Seconds())
				}

				// Server processing duration
				if server := result.ServerProcessingDuration(); server > 0 {
					t.config.DurationHistogram.With(prometheus.Labels{
						"phase":  "server",
						"method": method,
						"host":   host,
						"code":   code,
						"status": status,
					}).Observe(server.Seconds())
				}

				// Data transfer duration
				if transfer := result.DataTransferDuration(); transfer > 0 {
					t.config.DurationHistogram.With(prometheus.Labels{
						"phase":  "transfer",
						"method": method,
						"host":   host,
						"code":   code,
						"status": status,
					}).Observe(transfer.Seconds())
				}
			}
		}
	}

	return resp, err
}

// DefaultPrometheusHistogram creates a default histogram for HTTP client phase durations.
func DefaultPrometheusHistogram() *prometheus.HistogramVec {
	return prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "http_client_phase_duration_seconds",
			Help: "Duration of HTTP client request phases in seconds",
			Buckets: []float64{
				0.001, // 1ms
				0.005, // 5ms
				0.01,  // 10ms
				0.025, // 25ms
				0.05,  // 50ms
				0.1,   // 100ms
				0.25,  // 250ms
				0.5,   // 500ms
				1.0,   // 1s
				2.5,   // 2.5s
				5.0,   // 5s
				10.0,  // 10s
			},
		},
		[]string{"phase", "method", "host", "code", "status"},
	)
}

// DefaultPrometheusCounter creates a default counter for HTTP client requests.
func DefaultPrometheusCounter() *prometheus.CounterVec {
	return prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_client_requests_total",
			Help: "Total number of HTTP client requests",
		},
		[]string{"method", "host", "code", "status"},
	)
}

// DefaultPrometheusInFlightGauge creates a default gauge for in-flight requests.
func DefaultPrometheusInFlightGauge() prometheus.Gauge {
	return prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "http_client_in_flight_requests",
		Help: "Number of HTTP client requests currently in flight",
	})
}

// SimplePrometheusConfig creates a simple Prometheus configuration with common defaults.
func SimplePrometheusConfig() PrometheusConfig {
	hist := DefaultPrometheusHistogram()
	counter := DefaultPrometheusCounter()
	gauge := DefaultPrometheusInFlightGauge()

	// Register metrics
	prometheus.MustRegister(hist, counter, gauge)

	return PrometheusConfig{
		DurationHistogram: hist,
		RequestCounter:    counter,
		InFlightGauge:     gauge,
		DetailedMetrics:   true,
	}
}

// MustRegisterPrometheusMetrics is a helper to register Prometheus metrics with proper error handling.
func MustRegisterPrometheusMetrics(config PrometheusConfig) {
	if config.DurationHistogram != nil {
		prometheus.MustRegister(config.DurationHistogram)
	}
	if config.RequestCounter != nil {
		prometheus.MustRegister(config.RequestCounter)
	}
	if config.InFlightGauge != nil {
		prometheus.MustRegister(config.InFlightGauge)
	}
}

// UnregisterPrometheusMetrics unregisters Prometheus metrics (useful for testing).
func UnregisterPrometheusMetrics(config PrometheusConfig) {
	if config.DurationHistogram != nil {
		prometheus.Unregister(config.DurationHistogram)
	}
	if config.RequestCounter != nil {
		prometheus.Unregister(config.RequestCounter)
	}
	if config.InFlightGauge != nil {
		prometheus.Unregister(config.InFlightGauge)
	}
}

// WithSimplePrometheus is a convenience option that sets up Prometheus with sensible defaults.
func WithSimplePrometheus() Option {
	return func(f *Ferret) {
		// Create metrics but don't register them - let the user decide
		hist := prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name: fmt.Sprintf("ferret_http_duration_seconds_%d", time.Now().Unix()),
				Help: "Duration of HTTP request phases in seconds",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"phase", "method", "host", "code", "status"},
		)

		config := PrometheusConfig{
			DurationHistogram: hist,
			DetailedMetrics:   true,
		}

		// Wrap the transport
		f.next = &prometheusTransport{
			next:   f.next,
			config: config,
			ferret: f,
		}
	}
}