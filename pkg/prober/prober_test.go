package prober

import (
	"github.com/stretchr/testify/assert"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestProber(t *testing.T) {
	p := Prober{}
	p.Ok()
	assert.Equal(t, nil, p.IsOk())
	p.NotOk(ErrorDefault)
	assert.Equal(t, ErrorDefault, p.IsOk())
	p.Ok()
	assert.Equal(t, nil, p.IsOk())
}

func TestProber_HandleFunc(t *testing.T) {
	p := Prober{}
	req, err := http.NewRequest("GET", "/liveness", nil)
	if err != nil {
		t.Fatal(err)
	}
	handler := http.HandlerFunc(p.HandleFunc)

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)

	rr = httptest.NewRecorder()
	p.NotOk(ErrorDefault)
	handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusServiceUnavailable, rr.Code)

	rr = httptest.NewRecorder()
	p.Ok()
	handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)

}
