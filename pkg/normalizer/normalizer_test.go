package normalizer

import (
	"github.com/stretchr/testify/assert"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/producer"
	"net/url"
	"testing"
)

type testUrl struct {
	event      producer.RequestEvent
	expectedId string
}

func urlMustParse(u string) *url.URL {
	parsed, _ := url.Parse(u)
	return parsed
}

var testCases = []testUrl{
	testUrl{event: producer.RequestEvent{URL: urlMustParse("http://foo.bar:8845/foo/bar?param[]=a&param[]=b"), Method: "GET"}, expectedId: "GET:/foo/bar"},
	testUrl{event: producer.RequestEvent{URL: urlMustParse("http://foo.bar:8845/foo/bar?operationName=testOperation1&operationName=testOperation2"), Method: "GET"}, expectedId: "GET:/foo/bar:testOperation1:testOperation2"},
	testUrl{event: producer.RequestEvent{URL: urlMustParse("http://foo.bar:8845/foo/1587/bar"), Method: "GET"}, expectedId: "GET:/foo/0/bar"},
	testUrl{event: producer.RequestEvent{URL: urlMustParse("http://foo.bar:8845/user/10"), Method: "GET"}, expectedId: "GET:/user/0"},
	testUrl{event: producer.RequestEvent{URL: urlMustParse("http://foo.bar:8845/api/v1/bar"), Method: "GET"}, expectedId: "GET:/api/v1/bar"},
	testUrl{event: producer.RequestEvent{URL: urlMustParse("http://foo.bar:8845"), Method: "POST"}, expectedId: "POST:"},
	testUrl{event: producer.RequestEvent{URL: urlMustParse("http://foo.bar:8845/"), Method: ""}, expectedId: ":/"},
}

func TestRequestNormalizer_Run(t *testing.T) {
	normalizer := requestNormalizer{}
	for _, testCase := range testCases {
		assert.Equal(t, testCase.expectedId, normalizer.getNormalizedEventKey(&testCase.event))
	}
}
