package storage

import (
	"container/list"
	"sync"
)

// NewInMemoryCappedContainer create new in-memory capped container with capacity limit.
func NewInMemoryCappedContainer(capacity int) CappedContainer {
	return &inMemoryCappedContainer{
		list:     list.New(),
		capacity: capacity,
	}
}

type inMemoryCappedContainer struct {
	list     *list.List
	capacity int
	lock     sync.RWMutex
}

// Len returns current size of container.
func (h *inMemoryCappedContainer) Len() int {
	h.lock.RLock()
	defer h.lock.RUnlock()
	return h.list.Len()
}

// Capacity returns maximum limit of the capped container.
func (h *inMemoryCappedContainer) Capacity() int {
	return h.capacity
}

// Add adds new item to container.
func (h *inMemoryCappedContainer) Add(record interface{}) {
	h.lock.Lock()
	defer h.lock.Unlock()
	h.list.PushFront(record)

	// Drop items exceeding capacity limit.
	if h.list.Len() > h.capacity {
		h.list.Remove(h.list.Back())
	}
}

// Stream writes items to returned channel.
func (h *inMemoryCappedContainer) Stream() <-chan interface{} {
	stream := make(chan interface{})
	go func() {
		h.lock.RLock()
		defer h.lock.RUnlock()
		for e := h.list.Front(); e != nil; e = e.Next() {
			stream <- e.Value
		}
		close(stream)
	}()
	return stream
}
