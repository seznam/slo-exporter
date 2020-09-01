// +build !race

package storage

import (
	"context"
	"github.com/benbjohnson/clock"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"runtime"
	"time"
)

import (
	"fmt"
	"reflect"
	"testing"
)

type (
	value  interface{}
	values []value
)

func NewMockedTicker(mock *clock.Mock, interval time.Duration) Ticker {
	return &mockedTicker{
		mock:     mock,
		interval: interval,
	}
}

type mockedTicker struct {
	mock     *clock.Mock
	interval time.Duration
	ticker   *clock.Ticker
}

func (m *mockedTicker) C() <-chan time.Time {
	if m.ticker == nil {
		m.Start()
	}
	return m.ticker.C
}

func (m *mockedTicker) Start() {
	m.ticker = m.mock.Ticker(m.interval)
}

func (m *mockedTicker) Stop() {
	m.ticker.Stop()
}

func getTestingContainer(items values, capacity int) Container {
	c := NewInMemoryCappedContainer(capacity)
	for i := range items {
		c.Add(i)
	}
	return c
}

func Test_PeriodicalAggregatingArchiver_Run(t *testing.T) {
	t.Run("test archiver start and stop", func(t *testing.T) {
		c := getTestingContainer(values{}, 0)
		timeMock := clock.NewMock()
		archiver := NewPeriodicalAggregatingArchiver(logrus.New(), c, nil, nil, NewMockedTicker(timeMock, time.Second))
		ctx, cancel := context.WithCancel(context.Background())
		archiver.Run(ctx)
		cancel()
	})
}

func Test_PeriodicalAggregatingArchiver_Default_Current(t *testing.T) {
	t.Run("test if default value is set as current on start", func(t *testing.T) {
		defaultValue := 1
		c := getTestingContainer(values{}, 0)
		timeMock := clock.NewMock()
		archiver := NewPeriodicalAggregatingArchiver(logrus.New(), c, defaultValue, nil, NewMockedTicker(timeMock, time.Second))
		assert.Equal(t, defaultValue, archiver.Current())
	})
}

func Test_PeriodicalAggregatingArchiver_SetCurrent(t *testing.T) {
	t.Run("test if archiver updates current ", func(t *testing.T) {
		defaultValue := 0
		newValue := 20
		c := getTestingContainer(values{}, 0)
		timeMock := clock.NewMock()
		archiver := NewPeriodicalAggregatingArchiver(logrus.New(), c, defaultValue, nil, NewMockedTicker(timeMock, time.Second))
		assert.Equal(t, defaultValue, archiver.Current())
		archiver.SetCurrent(newValue)
		assert.Equal(t, newValue, archiver.Current())
	})
}

func sum(items []interface{}) (interface{}, error) {
	sum := 0
	for _, i := range items {
		number, ok := i.(int)
		if !ok {
			return nil, fmt.Errorf("unsupported item type: %s", reflect.TypeOf(i))
		}
		sum += number
	}
	return sum, nil
}

func Test_PeriodicalAggregatingArchiver_aggregate(t *testing.T) {
	t.Run("test history aggregation", func(t *testing.T) {
		var items values
		expectedAggregation := 0
		for i := 0; i < 3; i++ {
			items = append(items, 1)
			expectedAggregation++
		}
		defaultValue := 0
		c := getTestingContainer(values{1, 1, 1}, 3)
		timeMock := clock.NewMock()
		archiver := NewPeriodicalAggregatingArchiver(logrus.New(), c, defaultValue, sum, NewMockedTicker(timeMock, time.Second))
		assert.Equal(t, defaultValue, archiver.AggregatedHistory())
		if err := archiver.aggregate(); err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, expectedAggregation, archiver.AggregatedHistory())
	})
}

func Test_PeriodicalAggregatingArchiver_Complex(t *testing.T) {
	tests := []struct {
		name                string
		capacity            int
		interval            time.Duration
		wait                time.Duration
		defaultValue        value
		aggregationFunction aggregationFunction
		inputValues         values
		expectedAggregation value
	}{
		{name: "default aggregation with no inputs", capacity: 1, interval: time.Second, defaultValue: 0, aggregationFunction: sum, inputValues: values{}, expectedAggregation: 0},
		{name: "aggregate default values with no value set", capacity: 100, interval: time.Second, defaultValue: 1, aggregationFunction: sum, inputValues: values{}, expectedAggregation: 3, wait: 3 * time.Second},
		{name: "aggregate set values with capacity limit", capacity: 3, interval: time.Second, defaultValue: 0, aggregationFunction: sum, inputValues: values{1, 1, 1, 1, 1}, expectedAggregation: 3},
		{name: "aggregate set values without capacity limit", capacity: 100, interval: time.Second, defaultValue: 0, aggregationFunction: sum, inputValues: values{1, 1, 1, 1, 1}, expectedAggregation: 5},
	}
	timeMock := clock.NewMock()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := getTestingContainer(values{}, tt.capacity)
			archiver := NewPeriodicalAggregatingArchiver(logrus.New(), c, tt.defaultValue, tt.aggregationFunction, NewMockedTicker(timeMock, tt.interval))
			assert.Equal(t, tt.defaultValue, archiver.Current())
			assert.Equal(t, tt.defaultValue, archiver.AggregatedHistory())
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			archiver.Run(ctx)
			for _, i := range tt.inputValues {
				runtime.Gosched()
				archiver.SetCurrent(i)
				timeMock.Add(tt.interval)
			}
			runtime.Gosched()
			timeMock.Add(tt.wait)
			assert.Equal(t, tt.expectedAggregation, archiver.AggregatedHistory())
		})
	}

}
