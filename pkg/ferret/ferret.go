package ferret

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"net/http/httptrace"
	"time"
)

// contextKey is a custom type for context keys to avoid collisions.
type contextKey struct{}

// resultKey is the context key for storing Result.
var resultKey = contextKey{}

// Ferret is a custom HTTP transport that measures request timing.
// It is safe for concurrent use.
type Ferret struct {
	// The underlying transport to use. If nil, http.DefaultTransport is used.
	next http.RoundTripper

	// Options
	dialer              *net.Dialer
	disableKeepAlives   bool
	tlsHandshakeTimeout time.Duration

	// For testing
	clock func() time.Time
}

// NewFerret creates a new Ferret transport with default settings.
// DEPRECATED: Use New() with options instead.
func NewFerret() *Ferret {
	return New(
		WithKeepAlives(false),
		WithTimeout(2*time.Second, 0),
	)
}

// New creates a new Ferret transport with the given options.
func New(opts ...Option) *Ferret {
	f := &Ferret{
		clock:               time.Now,
		disableKeepAlives:   false, // Default to enabled for production use
		tlsHandshakeTimeout: 10 * time.Second,
		dialer: &net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		},
	}

	// Build the base transport first if not provided
	baseTransport := f.next
	if baseTransport == nil {
		baseTransport = &http.Transport{
			Proxy:               http.ProxyFromEnvironment,
			DialContext:         f.dialContext,
			TLSHandshakeTimeout: f.tlsHandshakeTimeout,
			DisableKeepAlives:   f.disableKeepAlives,
		}
		f.next = baseTransport
	}

	// Apply options after base transport is set
	for _, opt := range opts {
		opt(f)
	}

	// Fix up any wrapped transports that need the base transport
	if wrapper, ok := f.next.(*otelTransport); ok && wrapper.next == nil {
		wrapper.next = baseTransport
	}
	if wrapper, ok := f.next.(*prometheusTransport); ok && wrapper.next == nil {
		wrapper.next = baseTransport
	}

	return f
}

// RoundTrip implements http.RoundTripper.
// It measures the request timing and stores it in the request context.
func (f *Ferret) RoundTrip(req *http.Request) (*http.Response, error) {
	// Create a new result for this request
	result := &Result{
		Start: f.clock(),
	}

	// Attach result to context
	ctx := context.WithValue(req.Context(), resultKey, result)

	// Create httptrace client trace
	trace := &httptrace.ClientTrace{
		DNSStart: func(info httptrace.DNSStartInfo) {
			result.DNSStart = f.clock()
		},
		DNSDone: func(info httptrace.DNSDoneInfo) {
			result.DNSDone = f.clock()
		},
		ConnectStart: func(network, addr string) {
			result.ConnectStart = f.clock()
		},
		ConnectDone: func(network, addr string, err error) {
			result.ConnectDone = f.clock()
		},
		TLSHandshakeStart: func() {
			result.TLSHandshakeStart = f.clock()
		},
		TLSHandshakeDone: func(state tls.ConnectionState, err error) {
			result.TLSHandshakeDone = f.clock()
		},
		GotFirstResponseByte: func() {
			result.FirstByte = f.clock()
		},
	}

	// Add trace to context
	ctx = httptrace.WithClientTrace(ctx, trace)
	req = req.WithContext(ctx)

	// Execute the request
	resp, err := f.next.RoundTrip(req)

	// Record completion time
	result.End = f.clock()
	result.Error = err

	// If we got a response but FirstByte wasn't set (non-HTTP transport), set it now
	if resp != nil && result.FirstByte.IsZero() {
		result.FirstByte = result.End
	}

	// Store the result in the response request as well
	if resp != nil && resp.Request != nil {
		ctx := context.WithValue(resp.Request.Context(), resultKey, result)
		resp.Request = resp.Request.WithContext(ctx)
	}

	return resp, err
}

// dialContext is our custom dial function.
func (f *Ferret) dialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	// Simply dial - httptrace will handle the timing
	return f.dialer.DialContext(ctx, network, addr)
}

// GetResult retrieves the timing result from a request.
// It returns nil if no timing information is available.
func GetResult(req *http.Request) *Result {
	if req == nil {
		return nil
	}
	return resultFromContext(req.Context())
}

// resultFromContext retrieves the result from a context.
func resultFromContext(ctx context.Context) *Result {
	if ctx == nil {
		return nil
	}
	result, _ := ctx.Value(resultKey).(*Result)
	return result
}

// Legacy compatibility methods
// These methods are DEPRECATED and will be removed in a future version.

// ReqDuration returns the request duration.
// DEPRECATED: Use Result(req).RequestDuration() instead.
func (f *Ferret) ReqDuration() time.Duration {
	// This method cannot work correctly in concurrent scenarios
	// Return 0 to indicate unavailable
	return 0
}

// ConnDuration returns the connection duration.
// DEPRECATED: Use Result(req).ConnectionDuration() instead.
func (f *Ferret) ConnDuration() time.Duration {
	// This method cannot work correctly in concurrent scenarios
	// Return 0 to indicate unavailable
	return 0
}

// Duration returns the total duration.
// DEPRECATED: Use Result(req).TotalDuration() instead.
func (f *Ferret) Duration() time.Duration {
	// This method cannot work correctly in concurrent scenarios
	// Return 0 to indicate unavailable
	return 0
}
