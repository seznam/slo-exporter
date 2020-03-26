package pipeline

import (
	"fmt"
	"github.com/gorilla/mux"
	"github.com/iancoleman/strcase"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/config"
	"strconv"
	"strings"
)

var (
	eventProcessingDurationSeconds = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "event_processing_duration_seconds",
			Help:    "Duration histogram of event processing per module.",
			Buckets: prometheus.ExponentialBuckets(0.0005, 5, 6),
		},
		[]string{"module"},
	)
)

func NewManager(moduleFactory moduleFactoryFunction, config *config.Config, logger *logrus.Entry) (*Manager, error) {
	manager := Manager{
		pipeline: []pipelineItem{},
		logger:   logger,
	}
	// Initialize the pipeline and link it together.
	for _, moduleName := range config.Pipeline {
		newPipelineItem, err := manager.newPipelineItem(moduleName, config, moduleFactory)
		if err != nil {
			return nil, fmt.Errorf("failed to create pipeline module: %w", err)
		}
		manager.observeModuleEventProcessingDuration(newPipelineItem)
		if err := manager.addModuleToPipeline(newPipelineItem); err != nil {
			return nil, err
		}
	}
	return &manager, nil
}

type pipelineItem struct {
	name   string
	module Module
}

type Manager struct {
	pipeline []pipelineItem
	config   config.Config
	logger   *logrus.Entry
}

func (m *Manager) StartPipeline() {
	m.logger.Info("starting pipeline... ")
	var pipelineSchema []string
	for _, pipelineItem := range m.pipeline {
		pipelineItem.module.Run()
		pipelineSchema = append(pipelineSchema, pipelineItem.name)
	}
	m.logger.Info("pipeline schema: " + strings.Join(pipelineSchema, " -> "))
	m.logger.Info("pipeline started")

}

func (m *Manager) StopPipeline() {
	m.pipeline[0].module.Stop()
}

func (m *Manager) Done() bool {
	for _, pipelineItem := range m.pipeline {
		if !pipelineItem.module.Done() {
			return false
		}
	}
	return true
}

func (m *Manager) observeModuleEventProcessingDuration(item pipelineItem) {
	observableModule, ok := item.module.(ObservableModule)
	if ok {
		observableModule.RegisterEventProcessingDurationObserver(eventProcessingDurationSeconds.WithLabelValues(item.name))
	}
}

func (m *Manager) RegisterPrometheusMetrics(rootRegistry prometheus.Registerer, wrappedRegistry prometheus.Registerer) error {
	if err := rootRegistry.Register(eventProcessingDurationSeconds); err != nil {
		return err
	}
	m.logger.Info("registering Prometheus metrics of pipeline modules")
	for _, m := range m.pipeline {
		promModule, ok := m.module.(PrometheusInstrumentedModule)
		if !ok {
			continue
		}
		wrappedRegistry := prometheus.WrapRegistererWithPrefix(m.name+"_", wrappedRegistry)
		if err := promModule.RegisterMetrics(rootRegistry, wrappedRegistry); err != nil {
			return fmt.Errorf("error registering metrics of module %s: %w", m.name, err)
		}
	}
	return nil
}

func (m *Manager) RegisterWebInterface(router *mux.Router) {
	for _, m := range m.pipeline {
		webInterfaceModule, ok := m.module.(WebInterfaceModule)
		if !ok {
			continue
		}
		webInterfaceModule.RegisterInMux(router.PathPrefix("/" + m.name).Subrouter())
	}
}

func isProducer(module Module) bool {
	switch module.(type) {
	case RawEventProducerModule:
		return true
	case SloEventProducerModule:
		return true
	}
	return false
}

func isIngester(module Module) bool {
	switch module.(type) {
	case RawEventIngesterModule:
		return true
	case SloEventIngesterModule:
		return true
	}
	return false
}

func linkModules(previous, next Module) error {
	// We can link only previous producer with next ingester.
	if !isProducer(previous) {
		return fmt.Errorf("trying to link to module %s which is not a producer", previous)
	}
	if !isIngester(next) {
		return fmt.Errorf("trying to link module %s to previous module but it is not an ingester", next)
	}
	// Check if event types of producer end ingester matches.
	switch previous.(type) {
	case RawEventProducerModule:
		nextRaw, ok := next.(RawEventIngesterModule)
		if !ok {
			return fmt.Errorf("trying to link raw event producer %s with slo event ingester %v", previous, next)
		}
		nextRaw.SetInputChannel(previous.(RawEventProducerModule).OutputChannel())
	case SloEventProducerModule:
		nextRaw, ok := next.(SloEventIngesterModule)
		if !ok {
			return fmt.Errorf("trying to link SLO event producer %s with raw event ingester %v", previous, next)
		}
		nextRaw.SetInputChannel(previous.(SloEventProducerModule).OutputChannel())
	}
	return nil
}

func (m *Manager) lastPipelineItem() pipelineItem {
	return m.pipeline[len(m.pipeline)-1]
}

func (m *Manager) linkModuleWithPipeline(nextModule Module) error {
	// If it is first module to be in the pipeline, just check it's not an ingester and add it there.
	if len(m.pipeline) == 0 {
		if isIngester(nextModule) {
			return fmt.Errorf("ingester module %s cannot be at the at the beginning of the pipeline", nextModule)
		}
		return nil
	}

	previousPipelineItem := m.lastPipelineItem()
	// Link modules together.
	if err := linkModules(previousPipelineItem.module, nextModule); err != nil {
		return fmt.Errorf("failed to link modules: %w", err)
	}
	return nil
}

func (m *Manager) addModuleToPipeline(newItem pipelineItem) error {
	if err := m.linkModuleWithPipeline(newItem.module); err != nil {
		return err
	}
	m.pipeline = append(m.pipeline, newItem)
	return nil
}

func (m *Manager) newItemName(moduleName string) string {
	iterator := 0
	newItemName := strcase.ToSnake(moduleName)
	for _, i := range m.pipeline {
		if i.name == newItemName {
			iterator++
			newItemName += strconv.Itoa(iterator)
		}
	}
	return newItemName
}

func (m *Manager) newPipelineItem(moduleName string, config *config.Config, factoryFunction moduleFactoryFunction) (pipelineItem, error) {
	newItemName := m.newItemName(moduleName)
	moduleConfig, err := config.ModuleConfig(moduleName)
	if err != nil {
		return pipelineItem{}, fmt.Errorf("failed to load configuration for module %s: %w", moduleName, err)
	}
	newModule, err := factoryFunction(moduleName, m.logger.WithField("component", newItemName), moduleConfig)
	if err != nil {
		return pipelineItem{}, fmt.Errorf("failed to initialize module %s from config: %w", moduleName, err)
	}
	return pipelineItem{
		name:   newItemName,
		module: newModule,
	}, nil
}
