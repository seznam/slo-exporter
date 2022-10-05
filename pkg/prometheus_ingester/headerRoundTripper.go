package prometheus_ingester

import "net/http"

type httpHeadersRoundTripper struct {
	headers     map[string]string
	roudTripper http.RoundTripper
}

// RoundTrip implements the http.RoundTripper interface.
func (h httpHeadersRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) {
	// The specification of http.RoundTripper says that it shouldn't mutate
	// the request so make a copy of req.Header since this is all that is
	// modified. https://pkg.go.dev/net/http#RoundTripper
	r2 := new(http.Request)
	*r2 = *r
	r2.Header = make(http.Header, len(r.Header)+len(h.headers))
	// copy existing headers
	for k, v := range r.Header {
		r2.Header[k] = v
	}

	// add new headers
	for k, v := range h.headers {
		r2.Header.Set(k, v)
	}

	r = r2
	return h.roudTripper.RoundTrip(r)
}
