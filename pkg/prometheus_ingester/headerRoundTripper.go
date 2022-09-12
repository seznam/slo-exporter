package prometheus_ingester

import "net/http"

type httpHeadersRoundTripper struct {
	headers map[string]string
	rt      http.RoundTripper
}

// RoundTrip implements the http.RoundTripper interface.
func (h httpHeadersRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) {
	// The specification of http.RoundTripper says that it shouldn't mutate
	// the request so make a copy of req.Header since this is all that is
	// modified. https://pkg.go.dev/net/http#RoundTripper
	r2 := new(http.Request)
	*r2 = *r
	r2.Header = make(http.Header)
	// copy existing headers
	for k, s := range r.Header {
		r2.Header[k] = s
	}

	// add new headers
	for k, v := range h.headers {
		r2.Header.Set(k, v)
	}

	r = r2
	return h.rt.RoundTrip(r)
}
