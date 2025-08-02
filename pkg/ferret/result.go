package ferret

import (
	"encoding/json"
	"time"
)

// Result holds all timing information for a single HTTP request.
// It is immutable after the request completes.
type Result struct {
	// Basic timings
	Start       time.Time
	ConnectDone time.Time
	FirstByte   time.Time
	End         time.Time

	// Extended timings (will be populated in Phase 2)
	DNSStart         time.Time
	DNSDone          time.Time
	ConnectStart     time.Time
	TLSHandshakeStart time.Time
	TLSHandshakeDone  time.Time

	// Error if the request failed
	Error error
}

// ConnectionDuration returns the time taken to establish the connection.
func (r *Result) ConnectionDuration() time.Duration {
	// Use ConnectStart if available (from httptrace), otherwise use Start
	start := r.ConnectStart
	if start.IsZero() {
		start = r.Start
	}
	
	if r.ConnectDone.IsZero() || start.IsZero() {
		return 0
	}
	return r.ConnectDone.Sub(start)
}

// RequestDuration returns the time from connection established to first byte.
func (r *Result) RequestDuration() time.Duration {
	if r.FirstByte.IsZero() || r.ConnectDone.IsZero() {
		return 0
	}
	return r.FirstByte.Sub(r.ConnectDone)
}

// TotalDuration returns the total time for the request.
func (r *Result) TotalDuration() time.Duration {
	if r.End.IsZero() || r.Start.IsZero() {
		return 0
	}
	return r.End.Sub(r.Start)
}

// DNSDuration returns the time taken for DNS resolution.
// Returns 0 if DNS timing is not available.
func (r *Result) DNSDuration() time.Duration {
	if r.DNSDone.IsZero() || r.DNSStart.IsZero() {
		return 0
	}
	return r.DNSDone.Sub(r.DNSStart)
}

// TLSDuration returns the time taken for TLS handshake.
// Returns 0 if TLS timing is not available.
func (r *Result) TLSDuration() time.Duration {
	if r.TLSHandshakeDone.IsZero() || r.TLSHandshakeStart.IsZero() {
		return 0
	}
	return r.TLSHandshakeDone.Sub(r.TLSHandshakeStart)
}

// TTFB returns the time to first byte from the start of the request.
func (r *Result) TTFB() time.Duration {
	if r.FirstByte.IsZero() || r.Start.IsZero() {
		return 0
	}
	return r.FirstByte.Sub(r.Start)
}

// ServerProcessingDuration returns the time the server took to process the request.
// This is the time from when the request was sent until the first byte was received.
func (r *Result) ServerProcessingDuration() time.Duration {
	// Find the end of connection setup (either TLS done or connect done)
	connEnd := r.TLSHandshakeDone
	if connEnd.IsZero() {
		connEnd = r.ConnectDone
	}
	
	if r.FirstByte.IsZero() || connEnd.IsZero() {
		return 0
	}
	return r.FirstByte.Sub(connEnd)
}

// DataTransferDuration returns the time taken to receive the response body.
// This is the time from first byte to the end of the response.
func (r *Result) DataTransferDuration() time.Duration {
	if r.End.IsZero() || r.FirstByte.IsZero() {
		return 0
	}
	return r.End.Sub(r.FirstByte)
}

// MarshalJSON implements json.Marshaler for easy JSON output.
func (r *Result) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		DNSMs      float64 `json:"dns_ms,omitempty"`
		ConnectMs  float64 `json:"connect_ms"`
		TLSMs      float64 `json:"tls_ms,omitempty"`
		TTFBMs     float64 `json:"ttfb_ms"`
		TotalMs    float64 `json:"total_ms"`
		RequestMs  float64 `json:"request_ms"`
		Error      string  `json:"error,omitempty"`
	}{
		DNSMs:     float64(r.DNSDuration()) / float64(time.Millisecond),
		ConnectMs: float64(r.ConnectionDuration()) / float64(time.Millisecond),
		TLSMs:     float64(r.TLSDuration()) / float64(time.Millisecond),
		TTFBMs:    float64(r.TTFB()) / float64(time.Millisecond),
		TotalMs:   float64(r.TotalDuration()) / float64(time.Millisecond),
		RequestMs: float64(r.RequestDuration()) / float64(time.Millisecond),
		Error:     errorString(r.Error),
	})
}

// String returns a human-readable representation of the result.
func (r *Result) String() string {
	if r.Error != nil {
		return "Error: " + r.Error.Error()
	}
	
	s := "total=" + r.TotalDuration().String()
	
	// Add DNS time if available
	if dns := r.DNSDuration(); dns > 0 {
		s += " dns=" + dns.String()
	}
	
	// Add connection time
	if conn := r.ConnectionDuration(); conn > 0 {
		s += " connect=" + conn.String()
	}
	
	// Add TLS time if available
	if tls := r.TLSDuration(); tls > 0 {
		s += " tls=" + tls.String()
	}
	
	// Add TTFB
	if ttfb := r.TTFB(); ttfb > 0 {
		s += " ttfb=" + ttfb.String()
	}
	
	return s
}

func errorString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}