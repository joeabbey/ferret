package ferret

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"
)

// BenchmarkFerretOverhead measures the overhead of Ferret vs standard transport.
func BenchmarkFerretOverhead(b *testing.B) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	defer server.Close()

	b.Run("Standard", func(b *testing.B) {
		client := &http.Client{
			Transport: &http.Transport{
				DisableKeepAlives: true,
			},
		}
		
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			resp, err := client.Get(server.URL)
			if err != nil {
				b.Fatal(err)
			}
			resp.Body.Close()
		}
	})

	b.Run("Ferret", func(b *testing.B) {
		ferret := New(WithKeepAlives(false))
		client := &http.Client{Transport: ferret}
		
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			resp, err := client.Get(server.URL)
			if err != nil {
				b.Fatal(err)
			}
			resp.Body.Close()
		}
	})

	b.Run("FerretWithResult", func(b *testing.B) {
		ferret := New(WithKeepAlives(false))
		client := &http.Client{Transport: ferret}
		
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			req, _ := http.NewRequest("GET", server.URL, nil)
			resp, err := client.Do(req)
			if err != nil {
				b.Fatal(err)
			}
			result := GetResult(resp.Request)
			if result == nil {
				b.Fatal("No result")
			}
			resp.Body.Close()
		}
	})
}

// BenchmarkConcurrentRequests measures performance under concurrent load.
func BenchmarkConcurrentRequests(b *testing.B) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate some processing
		time.Sleep(1 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	ferret := New() // Keep-alives enabled
	client := &http.Client{Transport: ferret}

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			req, _ := http.NewRequest("GET", server.URL, nil)
			resp, err := client.Do(req)
			if err != nil {
				b.Fatal(err)
			}
			resp.Body.Close()
		}
	})
}

// BenchmarkWithObservability measures overhead of observability features.
func BenchmarkWithObservability(b *testing.B) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	b.Run("Plain", func(b *testing.B) {
		ferret := New()
		client := &http.Client{Transport: ferret}
		
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			resp, _ := client.Get(server.URL)
			resp.Body.Close()
		}
	})

	b.Run("WithPrometheus", func(b *testing.B) {
		// Create Prometheus config but don't register (to avoid conflicts)
		config := PrometheusConfig{
			DurationHistogram: DefaultPrometheusHistogram(),
			DetailedMetrics:   true,
		}
		
		ferret := New(WithPrometheus(config))
		client := &http.Client{Transport: ferret}
		
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			resp, _ := client.Get(server.URL)
			resp.Body.Close()
		}
	})
}

// BenchmarkResultOperations measures Result method performance.
func BenchmarkResultOperations(b *testing.B) {
	now := time.Now()
	result := &Result{
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

	b.Run("GetResult", func(b *testing.B) {
		// Create a request with result in context
		req, _ := http.NewRequest("GET", "http://example.com", nil)
		ctx := context.WithValue(req.Context(), resultKey, result)
		req = req.WithContext(ctx)
		
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			r := GetResult(req)
			if r == nil {
				b.Fatal("No result")
			}
		}
	})

	b.Run("AllDurations", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = result.DNSDuration()
			_ = result.ConnectionDuration()
			_ = result.TLSDuration()
			_ = result.ServerProcessingDuration()
			_ = result.DataTransferDuration()
			_ = result.TTFB()
			_ = result.TotalDuration()
		}
	})
}

// BenchmarkMemoryAllocation measures memory allocations.
func BenchmarkMemoryAllocation(b *testing.B) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	ferret := New()
	client := &http.Client{Transport: ferret}

	b.ReportAllocs()
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		req, _ := http.NewRequest("GET", server.URL, nil)
		resp, err := client.Do(req)
		if err != nil {
			b.Fatal(err)
		}
		result := GetResult(resp.Request)
		if result == nil {
			b.Fatal("No result")
		}
		resp.Body.Close()
	}
}

// BenchmarkLargeResponse measures performance with large responses.
func BenchmarkLargeResponse(b *testing.B) {
	// Create 1MB response
	largeData := make([]byte, 1024*1024)
	for i := range largeData {
		largeData[i] = byte(i % 256)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", strconv.Itoa(len(largeData)))
		w.Write(largeData)
	}))
	defer server.Close()

	ferret := New()
	client := &http.Client{Transport: ferret}

	b.SetBytes(int64(len(largeData)))
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		resp, err := client.Get(server.URL)
		if err != nil {
			b.Fatal(err)
		}
		
		// Read entire response
		buf := make([]byte, 4096)
		for {
			_, err := resp.Body.Read(buf)
			if err != nil {
				break
			}
		}
		resp.Body.Close()
	}
}