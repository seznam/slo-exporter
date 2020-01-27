package normalizer

import (
	"github.com/stretchr/testify/assert"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/producer"
	"net/url"
	"testing"
)

type testUrl struct {
	event      producer.RequestEvent
	expectedID string
}

func urlMustParse(u string) *url.URL {
	parsed, _ := url.Parse(u)
	return parsed
}

var testCases = []testUrl{
	testUrl{event: producer.RequestEvent{URL: urlMustParse("http://foo.bar:8845/foo/bar?param[]=a&param[]=b"), Method: "GET"}, expectedID: "GET:/foo/bar"},
	testUrl{event: producer.RequestEvent{URL: urlMustParse("http://foo.bar:8845/foo/bar?operationName=testOperation1&operationName=testOperation2"), Method: "GET"}, expectedID: "GET:/foo/bar:testOperation1:testOperation2"},
	testUrl{event: producer.RequestEvent{URL: urlMustParse("http://foo.bar:8845/foo/1587/bar"), Method: "GET"}, expectedID: "GET:/foo/0/bar"},
	testUrl{event: producer.RequestEvent{URL: urlMustParse("http://foo.bar:8845/user/10"), Method: "GET"}, expectedID: "GET:/user/0"},
	testUrl{event: producer.RequestEvent{URL: urlMustParse("http://foo.bar:8845/api/v1/bar"), Method: "GET"}, expectedID: "GET:/api/v1/bar"},
	testUrl{event: producer.RequestEvent{URL: urlMustParse("http://foo.bar:8845"), Method: "POST"}, expectedID: "POST:/"},
	testUrl{event: producer.RequestEvent{URL: urlMustParse("http://foo.bar:8845/"), Method: ""}, expectedID: ":/"},
	testUrl{event: producer.RequestEvent{URL: urlMustParse("http://foo.bar:8845/banner-250x250.png"), Method: ""}, expectedID: ":/banner-0x0.png"},
	testUrl{event: producer.RequestEvent{URL: urlMustParse("http://foo.bar:8845/foo////bar"), Method: ""}, expectedID: ":/foo/bar"},
	testUrl{event: producer.RequestEvent{URL: urlMustParse("http://foo.bar:8845/foo/bar///"), Method: ""}, expectedID: ":/foo/bar"},
}

func TestRequestNormalizer_Run(t *testing.T) {
	normalizer := requestNormalizer{}
	for _, testCase := range testCases {
		assert.Equal(t, testCase.expectedID, normalizer.getNormalizedEventKey(&testCase.event))
	}
}
