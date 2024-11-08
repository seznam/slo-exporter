package pipeline

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/iancoleman/strcase"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/seznam/slo-exporter/pkg/config"
	"github.com/sirupsen/logrus"
)

var eventProcessingDurationSeconds = prometheus.NewHistogramVec(
	prometheus.HistogramOpts{
		Name:    "event_processing_duration_seconds",
		Help:    "Duration histogram of event processing per module.",
		Buckets: prometheus.ExponentialBuckets(0.0005, 5, 6),
	},
	[]string{"module"},
)

func NewManager(moduleFactory moduleFactoryFunction, cfg *config.Config, logger logrus.FieldLogger) (*Manager, error) {
	manager := Manager{
		pipeline: []pipelineItem{},
		logger:   logger,
	}
	// Initialize the pipeline and link it together.
	for _, moduleName := range cfg.Pipeline {
		newPipelineItem, err := manager.newPipelineItem(moduleName, cfg, moduleFactory)
		if err != nil {
			return nil, fmt.Errorf("failed to create pipeline module: %w", err)
		}
		manager.observeModuleEventProcessingDuration(newPipelineItem)
		if err := manager.addModuleToPipelineEnd(newPipelineItem); err != nil {
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
	logger   logrus.FieldLogger
}

func (m *Manager) StartPipeline() error {
	m.logger.Info("starting pipeline... ")

	if len(m.pipeline) == 0 {
		return fmt.Errorf("failed to execute the empty pipeline (no pipeline modules defined in the config)")
	}
	pipelineSchema := make([]string, 0, len(m.pipeline))
	for _, pipelineItem := range m.pipeline {
		pipelineItem.module.Run()
		pipelineSchema = append(pipelineSchema, pipelineItem.name)
	}
	m.logger.Info("pipeline schema: " + strings.Join(pipelineSchema, " -> "))
	m.logger.Info("pipeline started")
	return nil
}

func (m *Manager) StopPipeline(ctx context.Context) chan struct{} {
	m.pipeline[0].module.Stop()
	stoppedChan := make(chan struct{})
	go func() {
		for {
			select {
			case <-ctx.Done():
				m.logger.Warn("Shutdown context expired before pipeline managed to process all events.")
				return
			default:
				if m.Done() {
					close(stoppedChan)
					m.logger.Info("Pipeline finished processing all events and successfully stopped.")
					return
				}
			}
			time.Sleep(10 * time.Millisecond)
		}
	}()
	return stoppedChan
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

func (m *Manager) RegisterPrometheusMetrics(rootRegistry, wrappedRegistry prometheus.Registerer) error {
	if err := rootRegistry.Register(eventProcessingDurationSeconds); err != nil {
		return err
	}
	m.logger.Info("registering Prometheus metrics of pipeline modules")
	for _, m := range m.pipeline {
		promModule, ok := m.module.(PrometheusInstrumentedModule)
		if !ok {
			continue
		}
		wrappedRegistry := prometheus.WrapRegistererWithPrefix(strcase.ToSnake(m.name)+"_", wrappedRegistry)
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
	switch previousRaw := previous.(type) {
	case RawEventProducerModule:
		nextRaw, ok := next.(RawEventIngesterModule)
		if !ok {
			return fmt.Errorf("trying to link raw event producer %s with slo event ingester %s", previous, next)
		}
		nextRaw.SetInputChannel(previousRaw.OutputChannel())
	case SloEventProducerModule:
		nextRaw, ok := next.(SloEventIngesterModule)
		if !ok {
			return fmt.Errorf("trying to link SLO event producer %s with raw event ingester %s", previous, next)
		}
		nextRaw.SetInputChannel(previousRaw.OutputChannel())
	}
	return nil
}

func (m *Manager) lastPipelineItem() pipelineItem {
	return m.pipeline[len(m.pipeline)-1]
}

func (m *Manager) linkModuleWithPipelineEnd(nextModule Module) error {
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

func (m *Manager) addModuleToPipelineEnd(newItem pipelineItem) error {
	if err := m.linkModuleWithPipelineEnd(newItem.module); err != nil {
		return err
	}
	m.pipeline = append(m.pipeline, newItem)
	return nil
}

func (m *Manager) newPipelineItem(moduleName string, cfg *config.Config, factoryFunction moduleFactoryFunction) (pipelineItem, error) {
	moduleConfig, err := cfg.ModuleConfig(moduleName)
	if err != nil {
		return pipelineItem{}, fmt.Errorf("failed to load configuration for module %s: %w", moduleName, err)
	}
	newModule, err := factoryFunction(moduleName, m.logger.WithField("component", moduleName), moduleConfig)
	if err != nil {
		return pipelineItem{}, fmt.Errorf("failed to initialize module %s from config: %w", moduleName, err)
	}
	return pipelineItem{
		name:   moduleName,
		module: newModule,
	}, nil
}
