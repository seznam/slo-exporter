package pipeline

import (
	"context"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/seznam/slo-exporter/pkg/config"
	"github.com/seznam/slo-exporter/pkg/event"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"strings"
	"testing"
)

type testModule struct {
	done bool
}

func (t *testModule) RegisterMetrics(rootRegistry prometheus.Registerer, wrappedRegistry prometheus.Registerer) error {
	wrappedRegistry.MustRegister(prometheus.NewCounter(prometheus.CounterOpts{
		Name: "test",
		Help: "test",
	}))
	return nil
}

func (t *testModule) Run() {
	t.done = false
}

func (t *testModule) Stop() {
	t.done = true
}

func (t *testModule) Done() bool {
	return t.done
}

func (t *testModule) OutputChannel() chan event.Raw {
	return make(chan event.Raw)
}

func newEmptyManager() (*Manager, error) {
	manager, err := NewManager(testModuleFactory, &config.Config{Pipeline: []string{}}, logrus.New())
	if err != nil {
		return nil, err
	}
	return manager, nil
}

func newTestManager() (*Manager, error) {
	manager, err := newEmptyManager()
	if err != nil {
		return nil, err
	}
	if err = manager.addModuleToPipelineEnd(pipelineItem{name: "test_module", module: &testModule{}}); err != nil {
		return nil, err
	}
	return manager, nil
}

func TestManager_RegisterPrometheusMetrics(t *testing.T) {
	manager, err := newTestManager()
	assert.NoError(t, err)

	registry := prometheus.NewRegistry()
	err = manager.RegisterPrometheusMetrics(registry, registry)
	assert.NoError(t, err)

	expectedMetrics := `
# HELP test_module_test test
# TYPE test_module_test counter
test_module_test 0
`

	err = testutil.GatherAndCompare(registry, strings.NewReader(expectedMetrics))
	assert.NoError(t, err)
}

// Empty pipeline refuses to start
func TestManager_StartEmptyPipeline(t *testing.T) {
	manager, err := newEmptyManager()
	assert.NoError(t, err)
	err = manager.StartPipeline()
	assert.Error(t, err)
}

func TestManager_StartPipeline(t *testing.T) {
	manager, err := newTestManager()
	assert.NoError(t, err)
	manager.StartPipeline()
	assert.False(t, manager.Done())
}

func TestManager_StopPipeline(t *testing.T) {
	manager, err := newTestManager()
	assert.NoError(t, err)
	ctx := context.Background()
	<-manager.StopPipeline(ctx)
	assert.True(t, manager.Done())
}

func TestManager_linkModuleWithPipeline(t *testing.T) {
	tests := []struct {
		manager    Manager
		nextModule Module
		wantErr    bool
	}{
		{manager: Manager{pipeline: []pipelineItem{}}, nextModule: testRawProducer{}, wantErr: false},
		{manager: Manager{pipeline: []pipelineItem{}}, nextModule: testRawIngester{}, wantErr: true},
	}
	for _, tt := range tests {
		err := tt.manager.linkModuleWithPipelineEnd(tt.nextModule)
		assert.Equal(t, tt.wantErr, err != nil)
	}
}

func Test_isIngester(t *testing.T) {
	tests := []struct {
		module Module
		result bool
	}{
		{module: testSloIngester{}, result: true},
		{module: testSloProducer{}, result: false},
		{module: testRawProducer{}, result: false},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.result, isIngester(tt.module))
	}
}

func Test_isProducer(t *testing.T) {
	tests := []struct {
		module Module
		result bool
	}{
		{module: testSloIngester{}, result: false},
		{module: testSloProducer{}, result: true},
		{module: testRawProducer{}, result: true},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.result, isProducer(tt.module))
	}
}

func Test_linkModules(t *testing.T) {
	tests := []struct {
		previous Module
		next     Module
		wantErr  bool
	}{
		{previous: testRawProducer{}, next: testRawIngester{}, wantErr: false},
		{previous: testRawIngester{}, next: testRawProducer{}, wantErr: true},
		{previous: testRawIngester{}, next: testSloIngester{}, wantErr: true},
		{previous: testSloProducer{}, next: testRawIngester{}, wantErr: true},
		{previous: testSloProducer{}, next: testSloIngester{}, wantErr: false},
	}
	for _, tt := range tests {
		if err := linkModules(tt.previous, tt.next); (err != nil) != tt.wantErr {
			t.Errorf("linkModules() error = %v, wantErr %v", err, tt.wantErr)
		}
	}
}

func Test_newPipelineItem(t *testing.T) {
	tests := []struct {
		moduleName string
		config     *viper.Viper
		expErr     bool
	}{
		{moduleName: "foo", config: viper.New(), expErr: true},
		{moduleName: "testRawIngester", config: viper.New(), expErr: true},
	}

	for _, tt := range tests {
		m, err := newTestManager()
		assert.NoError(t, err)
		_, err = m.newPipelineItem(tt.moduleName, config.New(logrus.New()), testModuleFactory)
		assert.Equal(t, tt.expErr, err != nil)
	}
}
