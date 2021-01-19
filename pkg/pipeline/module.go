package pipeline

import (
	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/seznam/slo-exporter/pkg/event"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

type moduleFactoryFunction func(moduleName string, logger logrus.FieldLogger, conf *viper.Viper) (Module, error)

type ModuleConstructor func(viperConfig *viper.Viper) (Module, error)

type EventProcessingDurationObserver interface {
	Observe(float64)
}

type Module interface {
	Run()
	Stop()
	Done() bool
}

type ObservableModule interface {
	RegisterEventProcessingDurationObserver(observer EventProcessingDurationObserver)
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
	SetInputChannel(chan *event.Raw)
}

type RawEventIngesterModule interface {
	Module
	RawEventIngester
}

type RawEventProducer interface {
	OutputChannel() chan *event.Raw
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
