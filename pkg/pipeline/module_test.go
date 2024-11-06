package pipeline

import (
	"fmt"

	"github.com/seznam/slo-exporter/pkg/event"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

func testModuleFactory(moduleName string, _ logrus.FieldLogger, _ *viper.Viper) (Module, error) {
	switch moduleName {
	case "testRawIngester":
		return testRawIngester{}, nil
	case "testRawProducer":
		return testRawProducer{}, nil
	case "testSloIngester":
		return testSloIngester{}, nil
	case "testSloProducer":
		return testSloProducer{}, nil
	default:
		return nil, fmt.Errorf("unknown module %s", moduleName)
	}
}

type testRawIngester struct{}

func (t testRawIngester) Run() {}

func (t testRawIngester) Stop() {}

func (t testRawIngester) Done() bool {
	return false
}

func (t testRawIngester) SetInputChannel(chan *event.Raw) {}

type testRawProducer struct{}

func (t testRawProducer) Run() {}

func (t testRawProducer) Stop() {}

func (t testRawProducer) Done() bool {
	return false
}

func (t testRawProducer) OutputChannel() chan *event.Raw {
	return make(chan *event.Raw)
}

type testSloIngester struct{}

func (t testSloIngester) Run() {}

func (t testSloIngester) Stop() {}

func (t testSloIngester) Done() bool {
	return false
}

func (t testSloIngester) SetInputChannel(chan *event.Slo) {}

type testSloProducer struct{}

func (t testSloProducer) Run() {}

func (t testSloProducer) Stop() {}

func (t testSloProducer) Done() bool {
	return false
}

func (t testSloProducer) OutputChannel() chan *event.Slo {
	return make(chan *event.Slo)
}
