package ferret

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

// TestResultZeroValues verifies behavior with zero time values.
func TestResultZeroValues(t *testing.T) {
	r := &Result{}

	// All durations should return 0 for zero values
	if r.ConnectionDuration() != 0 {
		t.Error("Expected 0 connection duration for zero values")
	}
	if r.RequestDuration() != 0 {
		t.Error("Expected 0 request duration for zero values")
	}
	if r.TotalDuration() != 0 {
		t.Error("Expected 0 total duration for zero values")
	}
	if r.DNSDuration() != 0 {
		t.Error("Expected 0 DNS duration for zero values")
	}
	if r.TLSDuration() != 0 {
		t.Error("Expected 0 TLS duration for zero values")
	}
	if r.TTFB() != 0 {
		t.Error("Expected 0 TTFB for zero values")
	}
	if r.ServerProcessingDuration() != 0 {
		t.Error("Expected 0 server processing duration for zero values")
	}
	if r.DataTransferDuration() != 0 {
		t.Error("Expected 0 data transfer duration for zero values")
	}
}

// TestResultPartialValues verifies behavior with partial timing data.
func TestResultPartialValues(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name   string
		result *Result
		check  func(*Result) time.Duration
		want   time.Duration
	}{
		{
			name: "only start and end",
			result: &Result{
				Start: now,
				End:   now.Add(100 * time.Millisecond),
			},
			check: (*Result).TotalDuration,
			want:  100 * time.Millisecond,
		},
		{
			name: "connection without explicit start",
			result: &Result{
				Start:       now,
				ConnectDone: now.Add(50 * time.Millisecond),
			},
			check: (*Result).ConnectionDuration,
			want:  50 * time.Millisecond,
		},
		{
			name: "connection with explicit start",
			result: &Result{
				Start:        now,
				ConnectStart: now.Add(10 * time.Millisecond),
				ConnectDone:  now.Add(60 * time.Millisecond),
			},
			check: (*Result).ConnectionDuration,
			want:  50 * time.Millisecond,
		},
		{
			name: "TLS without connection",
			result: &Result{
				TLSHandshakeStart: now,
				TLSHandshakeDone:  now.Add(20 * time.Millisecond),
			},
			check: (*Result).TLSDuration,
			want:  20 * time.Millisecond,
		},
		{
			name: "server processing without TLS",
			result: &Result{
				ConnectDone: now,
				FirstByte:   now.Add(100 * time.Millisecond),
			},
			check: (*Result).ServerProcessingDuration,
			want:  100 * time.Millisecond,
		},
		{
			name: "server processing with TLS",
			result: &Result{
				ConnectDone:      now,
				TLSHandshakeDone: now.Add(20 * time.Millisecond),
				FirstByte:        now.Add(120 * time.Millisecond),
			},
			check: (*Result).ServerProcessingDuration,
			want:  100 * time.Millisecond,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.check(tt.result)
			if got != tt.want {
				t.Errorf("Got %v, want %v", got, tt.want)
			}
		})
	}
}

// TestResultJSONSerialization verifies JSON output format.
func TestResultJSONSerialization(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name   string
		result *Result
		want   map[string]interface{}
	}{
		{
			name: "complete result",
			result: &Result{
				Start:             now,
				DNSStart:          now.Add(10 * time.Millisecond),
				DNSDone:           now.Add(20 * time.Millisecond),
				ConnectStart:      now.Add(20 * time.Millisecond),
				TLSHandshakeStart: now.Add(30 * time.Millisecond),
				TLSHandshakeDone:  now.Add(50 * time.Millisecond),
				ConnectDone:       now.Add(50 * time.Millisecond),
				FirstByte:         now.Add(100 * time.Millisecond),
				End:               now.Add(150 * time.Millisecond),
			},
			want: map[string]interface{}{
				"dns_ms":     10.0,
				"connect_ms": 30.0,
				"tls_ms":     20.0,
				"ttfb_ms":    100.0,
				"total_ms":   150.0,
				"request_ms": 50.0,
			},
		},
		{
			name: "result with error",
			result: &Result{
				Start: now,
				End:   now.Add(50 * time.Millisecond),
				Error: context.DeadlineExceeded,
			},
			want: map[string]interface{}{
				"connect_ms": 0.0,
				"ttfb_ms":    0.0,
				"total_ms":   50.0,
				"request_ms": 0.0,
				"error":      "context deadline exceeded",
			},
		},
		{
			name: "minimal result",
			result: &Result{
				Start:       now,
				ConnectDone: now.Add(30 * time.Millisecond),
				FirstByte:   now.Add(80 * time.Millisecond),
				End:         now.Add(100 * time.Millisecond),
			},
			want: map[string]interface{}{
				"connect_ms": 30.0,
				"ttfb_ms":    80.0,
				"total_ms":   100.0,
				"request_ms": 50.0,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.result)
			if err != nil {
				t.Fatalf("Failed to marshal JSON: %v", err)
			}

			var got map[string]interface{}
			if err := json.Unmarshal(data, &got); err != nil {
				t.Fatalf("Failed to unmarshal JSON: %v", err)
			}

			for key, wantVal := range tt.want {
				gotVal, ok := got[key]
				if !ok {
					t.Errorf("Missing key %q in JSON output", key)
					continue
				}

				// For numeric values, allow small floating point differences
				if wantFloat, ok := wantVal.(float64); ok {
					if gotFloat, ok := gotVal.(float64); ok {
						if diff := wantFloat - gotFloat; diff > 0.01 || diff < -0.01 {
							t.Errorf("Key %q: got %v, want %v", key, gotFloat, wantFloat)
						}
					} else {
						t.Errorf("Key %q: expected float64, got %T", key, gotVal)
					}
				} else if wantVal != gotVal {
					t.Errorf("Key %q: got %v, want %v", key, gotVal, wantVal)
				}
			}

			// Check no unexpected fields (except dns_ms and tls_ms which may be 0)
			for key := range got {
				if _, expected := tt.want[key]; !expected && key != "dns_ms" && key != "tls_ms" {
					if val, ok := got[key].(float64); !ok || val != 0 {
						t.Errorf("Unexpected key %q with value %v", key, got[key])
					}
				}
			}
		})
	}
}

// TestResultString verifies string representation.
func TestResultString(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name        string
		result      *Result
		contains    []string
		notContains []string
	}{
		{
			name: "complete timing",
			result: &Result{
				Start:             now,
				DNSStart:          now.Add(10 * time.Millisecond),
				DNSDone:           now.Add(20 * time.Millisecond),
				ConnectStart:      now.Add(20 * time.Millisecond),
				TLSHandshakeStart: now.Add(30 * time.Millisecond),
				TLSHandshakeDone:  now.Add(50 * time.Millisecond),
				ConnectDone:       now.Add(50 * time.Millisecond),
				FirstByte:         now.Add(100 * time.Millisecond),
				End:               now.Add(150 * time.Millisecond),
			},
			contains: []string{"total=150ms", "dns=10ms", "connect=30ms", "tls=20ms", "ttfb=100ms"},
		},
		{
			name: "no DNS timing",
			result: &Result{
				Start:       now,
				ConnectDone: now.Add(30 * time.Millisecond),
				FirstByte:   now.Add(80 * time.Millisecond),
				End:         now.Add(100 * time.Millisecond),
			},
			contains:    []string{"total=100ms", "connect=30ms", "ttfb=80ms"},
			notContains: []string{"dns=", "tls="},
		},
		{
			name: "error result",
			result: &Result{
				Error: context.DeadlineExceeded,
			},
			contains: []string{"Error:", "context deadline exceeded"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.result.String()

			for _, want := range tt.contains {
				if !contains(got, want) {
					t.Errorf("String() = %q, want it to contain %q", got, want)
				}
			}

			for _, notWant := range tt.notContains {
				if contains(got, notWant) {
					t.Errorf("String() = %q, should not contain %q", got, notWant)
				}
			}
		})
	}
}

// TestResultEdgeCases verifies edge case handling.
func TestResultEdgeCases(t *testing.T) {
	now := time.Now()

	// Test with times in wrong order
	r := &Result{
		Start: now.Add(100 * time.Millisecond),
		End:   now,
	}

	// Should handle gracefully (return 0 or handle the negative duration)
	if d := r.TotalDuration(); d > 0 {
		t.Errorf("Expected non-positive duration for reversed times, got %v", d)
	}

	// Test with nil error
	r2 := &Result{
		Start: now,
		End:   now.Add(100 * time.Millisecond),
		Error: nil,
	}

	data, err := json.Marshal(r2)
	if err != nil {
		t.Fatalf("Failed to marshal result with nil error: %v", err)
	}

	// Should not include error field
	if contains(string(data), "error") {
		t.Error("JSON should not include error field when error is nil")
	}

	// Test String() with nil error
	str := r2.String()
	if contains(str, "Error:") {
		t.Error("String() should not include Error when error is nil")
	}
}

// BenchmarkResultMethods benchmarks Result method performance.
func BenchmarkResultMethods(b *testing.B) {
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

	b.Run("TotalDuration", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = r.TotalDuration()
		}
	})

	b.Run("AllDurations", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = r.DNSDuration()
			_ = r.ConnectionDuration()
			_ = r.TLSDuration()
			_ = r.ServerProcessingDuration()
			_ = r.DataTransferDuration()
			_ = r.TTFB()
			_ = r.TotalDuration()
		}
	})

	b.Run("MarshalJSON", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, _ = r.MarshalJSON()
		}
	})

	b.Run("String", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = r.String()
		}
	})
}
