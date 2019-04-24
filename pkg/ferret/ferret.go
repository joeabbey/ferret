package ferret

import (
	"net"
	"net/http"
	"time"
)

//Ferret - A custom transport which adds timing information to measure request duration
type Ferret struct {
	rtp       http.RoundTripper
	dialer    *net.Dialer
	connStart time.Time
	connEnd   time.Time
	reqStart  time.Time
	reqEnd    time.Time
}

//NewFerret - Create a new Ferret (custom transport)
func NewFerret() *Ferret {

	f := &Ferret{
		dialer: &net.Dialer{
			Timeout:   2 * time.Second,
			KeepAlive: -1 * time.Second,
		},
	}
	f.rtp = &http.Transport{
		Proxy:               http.ProxyFromEnvironment,
		Dial:                f.dial,
		TLSHandshakeTimeout: 10 * time.Second,
		DisableKeepAlives:   true,
	}
	return f
}

//RoundTrip - Meausure the full time from start to finish
func (f *Ferret) RoundTrip(r *http.Request) (*http.Response, error) {
	f.reqStart = time.Now()
	resp, err := f.rtp.RoundTrip(r)
	f.reqEnd = time.Now()
	return resp, err
}

func (f *Ferret) dial(network, addr string) (net.Conn, error) {
	f.connStart = time.Now()
	cn, err := f.dialer.Dial(network, addr)
	f.connEnd = time.Now()
	return cn, err
}

//ReqDuration - Get the time spent making the request
func (f *Ferret) ReqDuration() time.Duration {
	return f.Duration() - f.ConnDuration()
}

//ConnDuration - Get the time spent connecting to the endpoint
func (f *Ferret) ConnDuration() time.Duration {
	return f.connEnd.Sub(f.connStart)
}

//Duration - Get the overall time spent
func (f *Ferret) Duration() time.Duration {
	return f.reqEnd.Sub(f.reqStart)
}
