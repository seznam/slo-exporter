//revive:disable:var-naming
package statistical_classifier

//revive:enable:var-naming

import (
	"container/list"
	"sync"
)

type history struct {
	list    *list.List
	maxSize int
	lock    sync.Mutex
}

func newHistory(size int) *history {
	return &history{
		list:    list.New(),
		maxSize: size,
	}
}

// addWeight adds new record to history
func (h *history) add(record interface{}) {
	h.lock.Lock()
	defer h.lock.Unlock()
	h.list.PushFront(record)

	// remove last sample if we have reached history size limit
	if h.list.Len() > h.maxSize {
		h.list.Remove(h.list.Back())
	}
}

// streamList writes all history items to chanel which streamList returns
func (h *history) streamList() chan interface{} {
	stream := make(chan interface{})
	go func() {
		h.lock.Lock()
		defer h.lock.Unlock()
		for e := h.list.Front(); e != nil; e = e.Next() {
			stream <- e.Value
		}
		close(stream)
	}()
	return stream
}
