package prometheus_ingester

import "net/http"

type httpHeadersRoundTripper struct {
	headers     map[string]string
	roudTripper http.RoundTripper
}

// RoundTrip implements the http.RoundTripper interface.
func (h httpHeadersRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) {
	// We use RoundTripper to inject HTTP headers even though it is not advised,
	// but the Prometheus client does not allow us to do it otherwise.
	for k, v := range h.headers {
		r.Header.Set(k, v)
	}

	return h.roudTripper.RoundTrip(r)
}
