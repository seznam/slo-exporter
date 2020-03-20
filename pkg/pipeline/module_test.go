package pipeline

import (
	"fmt"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/event"
)

func testModuleFactory(moduleName string, logger *logrus.Entry, conf *viper.Viper) (Module, error) {
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

func (t testRawIngester) Run() {
	return
}

func (t testRawIngester) Stop() {
	return
}

func (t testRawIngester) Done() bool {
	return false
}

func (t testRawIngester) SetInputChannel(chan *event.HttpRequest) {
	return
}

type testRawProducer struct{}

func (t testRawProducer) Run() {
	return
}

func (t testRawProducer) Stop() {
	return
}

func (t testRawProducer) Done() bool {
	return false
}

func (t testRawProducer) OutputChannel() chan *event.HttpRequest {
	return make(chan *event.HttpRequest)
}

type testSloIngester struct{}

func (t testSloIngester) Run() {
	return
}

func (t testSloIngester) Stop() {
	return
}

func (t testSloIngester) Done() bool {
	return false
}

func (t testSloIngester) SetInputChannel(chan *event.Slo) {
	return
}

type testSloProducer struct{}

func (t testSloProducer) Run() {
	return
}

func (t testSloProducer) Stop() {
	return
}

func (t testSloProducer) Done() bool {
	return false
}

func (t testSloProducer) OutputChannel() chan *event.Slo {
	return make(chan *event.Slo)
}
