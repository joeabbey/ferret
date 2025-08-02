package ferret

import (
	"net"
	"net/http"
	"time"
)

// Option is a functional option for configuring Ferret.
type Option func(*Ferret)

// WithKeepAlives configures whether to use HTTP keep-alives.
// By default, keep-alives are enabled for better performance.
// Set to false for cleaner per-request measurements.
func WithKeepAlives(enabled bool) Option {
	return func(f *Ferret) {
		f.disableKeepAlives = !enabled
	}
}

// WithTimeout configures connection and total timeouts.
// If total is 0, no total timeout is set.
func WithTimeout(connect, total time.Duration) Option {
	return func(f *Ferret) {
		if f.dialer == nil {
			f.dialer = &net.Dialer{}
		}
		f.dialer.Timeout = connect
		if !f.disableKeepAlives {
			// Set keep-alive to match connection timeout for consistency
			f.dialer.KeepAlive = connect
		} else {
			// Disable keep-alive by setting negative value
			f.dialer.KeepAlive = -1 * time.Second
		}
	}
}

// WithTransport sets a custom base transport.
// This allows layering Ferret on top of existing transports.
func WithTransport(base http.RoundTripper) Option {
	return func(f *Ferret) {
		f.next = base
	}
}

// WithDialer sets a custom dialer.
// This allows full control over the connection establishment.
func WithDialer(dialer *net.Dialer) Option {
	return func(f *Ferret) {
		f.dialer = dialer
	}
}

// WithTLSHandshakeTimeout sets the TLS handshake timeout.
func WithTLSHandshakeTimeout(timeout time.Duration) Option {
	return func(f *Ferret) {
		f.tlsHandshakeTimeout = timeout
	}
}

// WithClock sets a custom clock function for testing.
// This allows deterministic testing of timing logic.
func WithClock(clock func() time.Time) Option {
	return func(f *Ferret) {
		f.clock = clock
	}
}