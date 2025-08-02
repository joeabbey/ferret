// Package ferret provides a production-ready HTTP instrumentation library
// that captures detailed timing metrics for HTTP requests.
//
// Ferret implements the http.RoundTripper interface, making it easy to integrate
// with existing HTTP clients. It provides detailed timing information including
// DNS lookup, TCP connection, TLS handshake, time to first byte (TTFB), and
// data transfer duration.
//
// Basic usage:
//
//	transport := ferret.New()
//	client := &http.Client{Transport: transport}
//
//	resp, err := client.Get("https://example.com")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer resp.Body.Close()
//
//	result := ferret.GetResult(resp.Request)
//	if result != nil {
//	    fmt.Printf("Total time: %v\n", result.TotalDuration())
//	    fmt.Printf("DNS lookup: %v\n", result.DNSDuration())
//	    fmt.Printf("Connection: %v\n", result.ConnectionDuration())
//	    fmt.Printf("TLS handshake: %v\n", result.TLSDuration())
//	    fmt.Printf("TTFB: %v\n", result.TTFB())
//	}
//
// The library is designed to be thread-safe and can be used concurrently
// across multiple goroutines. All timing information is stored in the
// request context, ensuring no race conditions.
//
// Configuration options are available through the functional options pattern:
//
//	transport := ferret.New(
//	    ferret.WithTimeout(5*time.Second, 30*time.Second),
//	    ferret.WithKeepAlives(false),
//	    ferret.WithPrometheus(prometheusConfig),
//	    ferret.WithOpenTelemetry(tracer, spanNameFormatter),
//	)
//
// For Prometheus integration:
//
//	histogramVec := prometheus.NewHistogramVec(
//	    prometheus.HistogramOpts{
//	        Name: "http_request_duration_seconds",
//	        Help: "HTTP request latency distributions.",
//	    },
//	    []string{"method", "host", "status"},
//	)
//
//	transport := ferret.New(
//	    ferret.WithPrometheus(&ferret.PrometheusConfig{
//	        DurationHistogram: histogramVec,
//	    }),
//	)
//
// For OpenTelemetry integration:
//
//	transport := ferret.New(
//	    ferret.WithOpenTelemetry(tracer, ferret.DefaultSpanNameFormatter),
//	)
//
// The library has minimal overhead and is suitable for production use,
// including high-traffic services.
package ferret

