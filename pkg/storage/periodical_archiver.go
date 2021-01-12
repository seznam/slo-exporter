package storage

import (
	"context"
	"fmt"
	"github.com/sirupsen/logrus"
	"sync"
	"time"
)

type aggregationFunction func(items []interface{}) (interface{}, error)

type Ticker interface {
	C() <-chan time.Time
	Stop()
	Start()
}

func NewTicker(interval time.Duration) Ticker {
	return &ticker{interval: interval}
}

type ticker struct {
	interval time.Duration
	ticker   *time.Ticker
}

func (t *ticker) C() <-chan time.Time {
	if t.ticker == nil {
		t.Start()
	}
	return t.ticker.C
}

func (t *ticker) Start() {
	t.ticker = time.NewTicker(t.interval)
}

func (t *ticker) Stop() {
	t.ticker.Stop()
}

func oldestItem(items []interface{}) (interface{}, error) {
	return items[0], nil
}

func NewPeriodicalAggregatingArchiver(logger logrus.FieldLogger, container Container, defaultValue interface{}, aggregationFunc aggregationFunction, archiveTicker Ticker) *PeriodicalAggregatingArchiver {
	if aggregationFunc == nil {
		aggregationFunc = oldestItem
	}
	return &PeriodicalAggregatingArchiver{
		ticker:          archiveTicker,
		logger:          logger,
		container:       container,
		recentValue:     defaultValue,
		aggregatedValue: defaultValue,
		defaultValue:    defaultValue,
		aggregationFunc: aggregationFunc,
	}
}

type PeriodicalAggregatingArchiver struct {
	ticker          Ticker
	logger          logrus.FieldLogger
	lock            sync.RWMutex
	container       Container
	recentValue     interface{}
	aggregatedValue interface{}
	defaultValue    interface{}
	aggregationFunc aggregationFunction
}

func (p *PeriodicalAggregatingArchiver) SetCurrent(item interface{}) {
	p.lock.Lock()
	defer p.lock.Unlock()
	p.recentValue = item
}

func (p *PeriodicalAggregatingArchiver) Current() interface{} {
	p.lock.RLock()
	defer p.lock.RUnlock()
	return p.recentValue
}

func (p *PeriodicalAggregatingArchiver) History() []interface{} {
	var items []interface{}
	for i := range p.container.Stream() {
		items = append(items, i)
	}
	return items
}

func (p *PeriodicalAggregatingArchiver) AggregatedHistory() interface{} {
	return p.aggregatedValue
}

func (p *PeriodicalAggregatingArchiver) archive() {
	p.lock.Lock()
	defer p.lock.Unlock()
	p.container.Add(p.recentValue)
	p.recentValue = p.defaultValue
}

func (p *PeriodicalAggregatingArchiver) aggregate() error {
	aggregatedValue, err := p.aggregationFunc(p.History())
	if err != nil {
		return fmt.Errorf("failed to aggregate container data: %w", err)
	}
	p.aggregatedValue = aggregatedValue
	return nil
}

func (p *PeriodicalAggregatingArchiver) Run(ctx context.Context) {
	go func() {
		p.ticker.Start()
		defer p.ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-p.ticker.C():
				p.archive()
				if err := p.aggregate(); err != nil {
					p.logger.WithField("err", err).Errorf("failed to update aggregated statistics from history")
				}
			}
		}
	}()
}
