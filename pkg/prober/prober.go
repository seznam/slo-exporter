package prober

import (
	"errors"
	"fmt"
	"net/http"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
)

var (
	ErrDefault = fmt.Errorf("initializing")

	status = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "probe_status",
			Help: "Status of the probes",
		},
		[]string{"probe"},
	)
)

// NewLiveness returns prober to be used as a liveness probe.
func NewLiveness(registry prometheus.Registerer, logger logrus.FieldLogger) (*Prober, error) {
	p, err := newProber(registry, logger, "liveness")
	if err != nil {
		return nil, err
	}
	p.Ok()
	return p, nil
}

// NewReadiness returns prober to be used as a readiness probe.
func NewReadiness(registry prometheus.Registerer, logger logrus.FieldLogger) (*Prober, error) {
	p, err := newProber(registry, logger, "readiness")
	if err != nil {
		return nil, err
	}
	p.NotOk(ErrDefault)
	return p, nil
}

func newProber(registry prometheus.Registerer, logger logrus.FieldLogger, name string) (*Prober, error) {
	p := Prober{
		name:      name,
		statusMtx: sync.Mutex{},
		logger:    logger,
	}
	if err := registry.Register(status); err != nil {
		if !errors.As(err, &prometheus.AlreadyRegisteredError{}) {
			return nil, err
		}
	}
	return &p, nil
}

// Prober is struct holding information about status.
type Prober struct {
	name      string
	status    error
	statusMtx sync.Mutex
	logger    logrus.FieldLogger
}

// Ok sets the Prober to correct status.
func (p *Prober) Ok() {
	p.setStatus(nil)
}

// NotOk sets the Prober to not ready status and specifies reason as an error.
func (p *Prober) NotOk(err error) {
	p.setStatus(err)
}

// IsOk returns reason why Prober is not ok. If it is it returns nil.
func (p *Prober) IsOk() error {
	p.statusMtx.Lock()
	defer p.statusMtx.Unlock()
	return p.status
}

// Allows to use Prober in HTTP life-cycle endpoints.
func (p *Prober) HandleFunc(w http.ResponseWriter, _ *http.Request) {
	if p.IsOk() != nil {
		http.Error(w, p.IsOk().Error(), http.StatusServiceUnavailable)
		return
	}
	if _, err := w.Write([]byte("OK")); err != nil {
		p.logger.Errorf("error writing response: %v", err)
	}
}

func (p *Prober) setStatus(err error) {
	p.statusMtx.Lock()
	defer p.statusMtx.Unlock()
	if p.status != nil && err == nil {
		p.logger.Infof("changing %s status to ok", p.name)
		status.WithLabelValues(p.name).Set(1)
	}
	if p.status == nil && err != nil {
		p.logger.Warnf("changing %s status to not ok, reason: %+v", p.name, err)
		status.WithLabelValues(p.name).Set(0)
	}
	p.status = err
}
