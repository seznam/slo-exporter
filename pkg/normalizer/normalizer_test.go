package normalizer

import (
	"github.com/stretchr/testify/assert"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/producer"
	"net/url"
	"testing"
)

type testURL struct {
	event      producer.RequestEvent
	expectedID string
}

func urlMustParse(u string) *url.URL {
	parsed, _ := url.Parse(u)
	return parsed
}

var testCases = []testURL{
	testURL{event: producer.RequestEvent{URL: urlMustParse("http://foo.bar:8845/foo/bar?param[]=a&param[]=b"), Method: "GET"}, expectedID: "GET:/foo/bar"},
	testURL{event: producer.RequestEvent{URL: urlMustParse("http://foo.bar:8845/foo/bar?operationName=testOperation1&operationName=testOperation2"), Method: "GET"}, expectedID: "GET:/foo/bar:testOperation1:testOperation2"},
	testURL{event: producer.RequestEvent{URL: urlMustParse("http://foo.bar:8845/foo/1587/bar"), Method: "GET"}, expectedID: "GET:/foo/0/bar"},
	testURL{event: producer.RequestEvent{URL: urlMustParse("http://foo.bar:8845/user/10"), Method: "GET"}, expectedID: "GET:/user/0"},
	testURL{event: producer.RequestEvent{URL: urlMustParse("http://foo.bar:8845/api/v1/bar"), Method: "GET"}, expectedID: "GET:/api/v1/bar"},
	testURL{event: producer.RequestEvent{URL: urlMustParse("http://foo.bar:8845"), Method: "POST"}, expectedID: "POST:/"},
	testURL{event: producer.RequestEvent{URL: urlMustParse("http://foo.bar:8845/"), Method: ""}, expectedID: ":/"},
	testURL{event: producer.RequestEvent{URL: urlMustParse("http://foo.bar:8845/banner-250x250.png"), Method: ""}, expectedID: ":/:image"},
	testURL{event: producer.RequestEvent{URL: urlMustParse("http://foo.bar:8845/banner-250x250.info"), Method: ""}, expectedID: ":/banner-0x0.info"},
	testURL{event: producer.RequestEvent{URL: urlMustParse("http://foo.bar:8845/foo////bar"), Method: ""}, expectedID: ":/foo/bar"},
	testURL{event: producer.RequestEvent{URL: urlMustParse("http://foo.bar:8845/foo/bar///"), Method: ""}, expectedID: ":/foo/bar"},
	testURL{event: producer.RequestEvent{URL: urlMustParse("http://foo.bar:8845/api/v1/ppchit/rule/0decf0c0cfb0"), Method: ""}, expectedID: ":/api/v1/ppchit/rule/0"},
	testURL{event: producer.RequestEvent{URL: urlMustParse("http://foo.bar:8845/api/v1/ppchit/rule/0decxxxc0cfb0"), Method: ""}, expectedID: ":/api/v1/ppchit/rule/0decxxxc0cfb0"},
	testURL{event: producer.RequestEvent{URL: urlMustParse("http://foo.bar:8845/campaigns/0/groups/0/placements/automatic/fusakla.cz/urls"), Method: ""}, expectedID: ":/campaigns/0/groups/0/placements/automatic/:domain/urls"},
	testURL{event: producer.RequestEvent{URL: urlMustParse("http://foo.bar:8845/foo/127.0.0.1/bar"), Method: ""}, expectedID: ":/foo/:ip/bar"},
	testURL{event: producer.RequestEvent{URL: urlMustParse("http://foo.bar:8845/md5/098f6bcd4621d373cade4e832627b4f6/bar"), Method: ""}, expectedID: ":/md5/:hash/bar"},
	testURL{event: producer.RequestEvent{URL: urlMustParse("http://foo.bar:8845/uuid4/dde8645e-a78a-4833-926a-936eb7481a5c/bar"), Method: ""}, expectedID: ":/uuid4/:uuid/bar"},
	testURL{event: producer.RequestEvent{URL: urlMustParse("http://foo.bar:8845/campaigns/111/groups/254/fonts/Roboto-Regular.ttf"), Method: ""}, expectedID: ":/campaigns/0/groups/0/fonts/:font"},
}


func TestRequestNormalizer_Run(t *testing.T) {
	normalizer := requestNormalizer{}
	for _, testCase := range testCases {
		assert.Equal(t, testCase.expectedID, normalizer.getNormalizedEventKey(&testCase.event))
	}
}
