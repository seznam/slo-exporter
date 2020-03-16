package shutdown_handler

import (
	"context"
	log "github.com/sirupsen/logrus"
	"sync"
)

type GracefulShutdownHandler struct {
	ProducersContextWithCancel  context.Context
	shutdownWaitGroup           *sync.WaitGroup
	gracefulShutdownRequestChan chan struct{}
}

func New(producersContext context.Context, gracefulShutdownRequestChan chan struct{}) GracefulShutdownHandler {
	var wg sync.WaitGroup
	return GracefulShutdownHandler{
		ProducersContextWithCancel:  producersContext,
		shutdownWaitGroup:           &wg,
		gracefulShutdownRequestChan: gracefulShutdownRequestChan,
	}
}

func (g GracefulShutdownHandler) RequestShutdownIfAllJobsAreDone() {
	go func() {
		g.shutdownWaitGroup.Wait()
		g.gracefulShutdownRequestChan <- struct{}{}
	}()
}

func (g GracefulShutdownHandler) Done() {
	g.shutdownWaitGroup.Done()
}

func (g GracefulShutdownHandler) Inc() {
	g.shutdownWaitGroup.Add(1)
}

func (g GracefulShutdownHandler) Wait(shutdownCtx context.Context) {
	log.Infof("waiting configured graceful shutdown timeout")

	// Now wait for what happens first (either timeout or wait group finishes)
	waitGroupDone := make(chan struct{})
	go func() {
		defer close(waitGroupDone)
		g.shutdownWaitGroup.Wait()
	}()

	select {
	case <-waitGroupDone:
		log.Info("all processes finished voluntarily, what a respect")
	case <-shutdownCtx.Done():
		log.Warn("time's up! gonna kill everyone who didn't finish until now")
	}
}
