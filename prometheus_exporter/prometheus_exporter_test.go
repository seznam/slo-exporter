package prometheus_exporter

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

type testNormalizeEventMetadata struct {
	knownMetadata  []string
	input          map[string]string
	expectedOutput map[string]string
}

func Test_normalizeEventMetadata(t *testing.T) {
	testCases := []testNormalizeEventMetadata{
		testNormalizeEventMetadata{
			knownMetadata:  []string{"b", "c"},
			input:          map[string]string{"b": "b", "c": "c"},
			expectedOutput: map[string]string{"b": "b", "c": "c"},
		},
		testNormalizeEventMetadata{
			knownMetadata:  []string{"b", "c"},
			input:          map[string]string{"a": "a", "b": "b", "c": "c"},
			expectedOutput: map[string]string{"b": "b", "c": "c"},
		},
		testNormalizeEventMetadata{
			knownMetadata:  []string{"b", "c"},
			input:          map[string]string{"c": "c", "d": "d"},
			expectedOutput: map[string]string{"b": "", "c": "c"},
		},
		testNormalizeEventMetadata{
			knownMetadata:  []string{"b", "c"},
			input:          map[string]string{"d": "d", "e": "e"},
			expectedOutput: map[string]string{"b": "", "c": ""},
		},
	}
	for _, testCase := range testCases {
		assert.Equal(t, testCase.expectedOutput, normalizeEventMetadata(testCase.knownMetadata, testCase.input))
	}
}
