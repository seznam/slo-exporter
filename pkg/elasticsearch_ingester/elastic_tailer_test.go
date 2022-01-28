package elasticsearch_ingester

import (
	"context"
	"encoding/json"
	"github.com/seznam/slo-exporter/pkg/elasticsearch_client"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"regexp"
	"testing"
	"time"
)

func newJsonBody(document map[string]string) json.RawMessage {
	data, err := json.Marshal(document)
	if err != nil {
		panic(err)
	}
	return data
}

const (
	timestampFormat = "2006-01-02T15:04:05"
)

var (
	defaultTimestamp = time.Time{}
)

func newTimestamp(t string) time.Time {
	timestamp, err := time.Parse(timestampFormat, t)
	if err != nil {
		panic(err)
	}
	return timestamp
}

type doc map[string]string

func Test_tailer_newDocumentFromJson(t *testing.T) {
	tests := []struct {
		name                   string
		data                   json.RawMessage
		returnedError          error
		timestampField         string
		timestampFormat        string
		rawLogFiled            string
		rawLogParseRegexp      *regexp.Regexp
		rawLogEmptyGroupRegexp *regexp.Regexp
		expectedError          bool
		expectedDocument       document
	}{
		{
			name:                   "successfully parse document event with timestamp, raw log and empty group ",
			data:                   newJsonBody(doc{"@timestamp": "2006-01-02T15:04:05", "log": `lvl="info" msg="foo bar" user-agent=""`}),
			returnedError:          nil,
			timestampField:         "@timestamp",
			timestampFormat:        timestampFormat,
			rawLogFiled:            "log",
			rawLogParseRegexp:      regexp.MustCompile(`lvl="(?P<lvl>[^"]*)" msg="(?P<msg>[^"]*)" user-agent="(?P<user_agent>[^"]*)"`),
			rawLogEmptyGroupRegexp: regexp.MustCompile("^$"),
			expectedError:          false,
			expectedDocument: document{
				timestamp: newTimestamp("2006-01-02T15:04:05"),
				fields:    map[string]string{"@timestamp": "2006-01-02T15:04:05", "log": `lvl="info" msg="foo bar" user-agent=""`, "lvl": "info", "msg": "foo bar"},
			},
		},
		{
			name:                   "successfully parse document event with timestamp, raw log and not matching empty group and normalized special chars",
			data:                   newJsonBody(doc{"@timestamp": "2006-01-02T15:04:05", "log": `lvl="info" msg="foo bar" user-agent=""`}),
			returnedError:          nil,
			timestampField:         "@timestamp",
			timestampFormat:        timestampFormat,
			rawLogFiled:            "log",
			rawLogParseRegexp:      regexp.MustCompile(`lvl="(?P<lvl>[^"]*)" msg="(?P<msg>[^"]*)" user-agent="(?P<user_agent>[^"]*)"`),
			rawLogEmptyGroupRegexp: regexp.MustCompile("^-$"),
			expectedError:          false,
			expectedDocument: document{
				timestamp: newTimestamp("2006-01-02T15:04:05"),
				fields:    map[string]string{"@timestamp": "2006-01-02T15:04:05", "log": `lvl="info" msg="foo bar" user-agent=""`, "lvl": "info", "msg": "foo bar", "user_agent": ""},
			},
		},
		{
			name:                   "successfully parse document event with timestamp, invalid raw log format",
			data:                   newJsonBody(doc{"@timestamp": "2006-01-02T15:04:05", "log": `msg="foo bar" user-agent=""`}),
			returnedError:          nil,
			timestampField:         "@timestamp",
			timestampFormat:        timestampFormat,
			rawLogFiled:            "log",
			rawLogParseRegexp:      regexp.MustCompile(`lvl="(?P<lvl>[^"]*)" msg="(?P<msg>[^"]*)" user-agent="(?P<user_agent>[^"]*)"`),
			rawLogEmptyGroupRegexp: regexp.MustCompile("^-$"),
			expectedError:          true,
		},
		{
			name:                   "successfully parse document event with timestamp, missing raw log field",
			data:                   newJsonBody(doc{"@timestamp": "2006-01-02T15:04:05"}),
			returnedError:          nil,
			timestampField:         "@timestamp",
			timestampFormat:        timestampFormat,
			rawLogFiled:            "log",
			rawLogParseRegexp:      regexp.MustCompile(`lvl="(?P<lvl>[^"]*)" msg="(?P<msg>[^"]*)" user-agent="(?P<user_agent>[^"]*)"`),
			rawLogEmptyGroupRegexp: regexp.MustCompile("^-$"),
			expectedError:          true,
		},
		{
			name:            "successfully parse document event with invalid timestamp format",
			data:            newJsonBody(doc{"@timestamp": "2006-01-02T  xxx  15:04:05"}),
			returnedError:   nil,
			timestampField:  "@timestamp",
			timestampFormat: timestampFormat,
			expectedError:   true,
		},
		{
			name:            "successfully parse document event with missing timestamp field",
			data:            newJsonBody(doc{}),
			returnedError:   nil,
			timestampField:  "@timestamp",
			timestampFormat: timestampFormat,
			expectedError:   true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testTailer := tailer{
				timestampField:         tt.timestampField,
				timestampFormat:        tt.timestampFormat,
				rawLogField:            tt.rawLogFiled,
				rawLogFormatRegexp:     tt.rawLogParseRegexp,
				rawLogEmptyGroupRegexp: tt.rawLogEmptyGroupRegexp,
				logger:                 logrus.New(),
			}
			got, err := testTailer.newDocumentFromJson(tt.data)
			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedDocument, got)
			}
		})
	}
}

func Test_tailer_run(t *testing.T) {
	ts := "2006-01-02T15:04:05"
	tst := newTimestamp(ts)
	testTailer := tailer{
		client:          elasticsearch_client.NewClientMock([]json.RawMessage{newJsonBody(doc{"@timestamp": ts})}, 0, nil),
		timestampField:  "@timestamp",
		timestampFormat: timestampFormat,
		lastTimestamp:   time.Time{},
		maxBatchSize:    1,
		logger:          logrus.New(),
	}
	expectedDocs := []document{
		{
			timestamp: tst,
			fields:    map[string]string{"@timestamp": ts},
		},
	}

	// Tailer should first query schedule immediately, the maxBatchSize is 1 and the query returns 2 documents
	// so the tailer should immediately schedule next query to catch up with the left documents' event though the interval is 1m
	// Next query should receive again 1 document and one left.
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	out := testTailer.run(ctx, time.Minute)
	var res []document
	for d := range out {
		res = append(res, d)
		cancel()
	}
	assert.Equal(t, tst, testTailer.lastTimestamp)
	assert.ElementsMatch(t, expectedDocs, res)
}
