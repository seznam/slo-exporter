package sqlwriter

import (
	"errors"
	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
	"regexp"
	"testing"
	"time"
)

var (
	testError     = errors.New("test error")
	interval      = time.Millisecond
	retryLimit    = 3
	queryResult   = sqlmock.NewResult(1, 1)
	waitForQueues = time.Second
)

func waitForZero(t *testing.T, f func() int, timeout time.Duration) {
	for _ = range time.NewTimer(timeout).C {
		if f() == 0 {
			return
		}
	}
	t.Error("timed out waiting for zero")
}

func newMockedBatchWriter(t *testing.T) (SqlWriter, sqlmock.Sqlmock) {
	logger := logrus.NewEntry(logrus.New())
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Error("error creating mock database")
	}
	w := NewSqlBatchWriter(db, logger, prometheus.NewRegistry(), 10, interval, interval, retryLimit)
	return w, mock
}

func TestBatchedSqlWriter_Write_AllOk(t *testing.T) {
	w, mock := newMockedBatchWriter(t)

	mock.ExpectBegin().WillReturnError(nil)
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO metrics VALUES (1)")).WillReturnResult(queryResult).WillReturnError(nil)
	mock.ExpectCommit().WillReturnError(nil)
	w.Write("INSERT INTO metrics VALUES (1)")

	waitForZero(t, w.WriteQueueSize, waitForQueues)
	waitForZero(t, w.RetryQueueSize, waitForQueues)
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Error(err)
		return
	}
}

func TestBatchedSqlWriter_Write_SuccessfulRetry(t *testing.T) {
	w, mock := newMockedBatchWriter(t)

	// First failed write
	mock.ExpectBegin().WillReturnError(nil)
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO metrics VALUES (1)")).WillReturnResult(queryResult).WillReturnError(testError)
	mock.ExpectCommit().WillReturnError(nil)
	// Successful retry
	mock.ExpectBegin().WillReturnError(nil)
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO metrics VALUES (1)")).WillReturnResult(queryResult).WillReturnError(nil)
	mock.ExpectCommit().WillReturnError(nil)

	w.Write("INSERT INTO metrics VALUES (1)")

	waitForZero(t, w.WriteQueueSize, waitForQueues)
	waitForZero(t, w.RetryQueueSize, waitForQueues)
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Error(err)
		return
	}
}

func TestBatchedSqlWriter_Write_FailedRetry(t *testing.T) {
	w, mock := newMockedBatchWriter(t)

	// First failed write
	mock.ExpectBegin().WillReturnError(nil)
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO metrics VALUES (1)")).WillReturnResult(queryResult).WillReturnError(testError)
	mock.ExpectCommit().WillReturnError(nil)
	// Failed retries
	for i := 0; i < retryLimit; i++ {
		mock.ExpectBegin().WillReturnError(nil)
		mock.ExpectExec(regexp.QuoteMeta("INSERT INTO metrics VALUES (1)")).WillReturnResult(queryResult).WillReturnError(testError)
		mock.ExpectCommit().WillReturnError(nil)
	}
	w.Write("INSERT INTO metrics VALUES (1)")

	waitForZero(t, w.WriteQueueSize, waitForQueues)
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Error(err)
		return
	}
}

func TestBatchedSqlWriter_Write_FailedCommit(t *testing.T) {
	w, mock := newMockedBatchWriter(t)

	// First failed write
	mock.ExpectBegin().WillReturnError(nil)
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO metrics VALUES (1)")).WillReturnResult(queryResult).WillReturnError(nil)
	mock.ExpectCommit().WillReturnError(testError)

	mock.ExpectBegin().WillReturnError(nil)
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO metrics VALUES (1)")).WillReturnResult(queryResult).WillReturnError(nil)
	mock.ExpectCommit().WillReturnError(nil)

	w.Write("INSERT INTO metrics VALUES (1)")

	waitForZero(t, w.WriteQueueSize, waitForQueues)
	waitForZero(t, w.RetryQueueSize, waitForQueues)
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Error(err)
		return
	}
}
