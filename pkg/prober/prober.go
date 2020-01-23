package prober

import (
	"fmt"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
	"net/http"
	"sync"
)

var (
	log          *logrus.Entry
	ErrorDefault = fmt.Errorf("initializing")

	status = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "probe_status",
			Help: "Status of the probes",
		},
		[]string{"probe"},
	)
)

func init() {
	log = logrus.WithField("component", "prober")
	prometheus.DefaultRegisterer.MustRegister(status)
}

// NewLiveness returns prober to be used as a liveness probe
func NewLiveness() *Prober {
	p := Prober{
		name:      "liveness",
		statusMtx: sync.Mutex{},
	}
	p.Ok()
	return &p
}

// NewReadiness returns prober to be used as a readiness rpobe
func NewReadiness() *Prober {
	p := Prober{
		name:      "readiness",
		statusMtx: sync.Mutex{},
	}
	p.NotOk(ErrorDefault)
	return &p
}

// Prober is struct holding information about status
type Prober struct {
	name      string
	status    error
	statusMtx sync.Mutex
}

// Ok sets the Prober to correct status
func (p *Prober) Ok() {
	p.setStatus(nil)
}

// NotOk sets the Prober to not ready status and specifies reason as an error
func (p *Prober) NotOk(err error) {
	p.setStatus(err)
}

// IsOk returns reason why Prober is not ok. If it is it returns nil.
func (p *Prober) IsOk() error {
	p.statusMtx.Lock()
	defer p.statusMtx.Unlock()
	return p.status
}

// Allows to use Prober in HTTP life-cycle endpoints
func (p *Prober) HandleFunc(w http.ResponseWriter, req *http.Request) {
	if p.IsOk() != nil {
		http.Error(w, p.IsOk().Error(), http.StatusServiceUnavailable)
		return
	}
	_, _ = w.Write([]byte("OK"))
}

func (p *Prober) setStatus(err error) {
	p.statusMtx.Lock()
	defer p.statusMtx.Unlock()
	if p.status != nil && err == nil {
		log.Infof("changing %s status to ok", p.name)
		status.WithLabelValues(p.name).Set(1)
	}
	if p.status == nil && err != nil {
		log.Warnf("changing %s status to not ok, reason: %v", p.name, err)
		status.WithLabelValues(p.name).Set(0)
	}
	p.status = err
}
