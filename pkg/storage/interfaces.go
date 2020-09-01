package storage

type Container interface {
	Add(item interface{})
	Stream() <-chan interface{}
	Len() int
}

type CappedContainer interface {
	Container
	Capacity() int
}
