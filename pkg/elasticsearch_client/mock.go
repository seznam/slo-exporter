package elasticsearch_client

import (
	"context"
	"encoding/json"
	"time"
)

func NewClientMock(data []json.RawMessage, documentsLeft int, err error) Client {
	return &clientMock{
		data:          data,
		documentsLeft: documentsLeft,
		error:         err,
	}
}

type clientMock struct {
	data          []json.RawMessage
	documentsLeft int
	error         error
}

func (c *clientMock) RangeSearch(ctx context.Context, index, timestampField string, since time.Time, size int, query string, timeout time.Duration) ([]json.RawMessage, int, error) {
	return c.data, c.documentsLeft, c.error
}
