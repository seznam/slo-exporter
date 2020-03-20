package pipeline

import (
	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/event"
)

type moduleFactoryFunction func(moduleName string, logger *logrus.Entry, conf *viper.Viper) (Module, error)

type ModuleConstructor func(viperConfig *viper.Viper) (Module, error)

type Module interface {
	Run()
	Stop()
	Done() bool
}

type PrometheusInstrumentedModule interface {
	Module
	RegisterMetrics(rootRegistry prometheus.Registerer, wrappedRegistry prometheus.Registerer) error
}

type WebInterfaceModule interface {
	Module
	RegisterInMux(router *mux.Router)
}

type RawEventIngester interface {
	SetInputChannel(chan *event.HttpRequest)
}

type RawEventIngesterModule interface {
	Module
	RawEventIngester
}

type RawEventProducer interface {
	OutputChannel() chan *event.HttpRequest
}

type RawEventProducerModule interface {
	Module
	RawEventProducer
}

type SloEventIngester interface {
	SetInputChannel(chan *event.Slo)
}

type SloEventIngesterModule interface {
	Module
	SloEventIngester
}

type SloEventProducer interface {
	OutputChannel() chan *event.Slo
}

type SloEventProducerModule interface {
	Module
	SloEventProducer
}

type ProcessorModule interface {
	Module
	RawEventIngester
	RawEventProducer
}
