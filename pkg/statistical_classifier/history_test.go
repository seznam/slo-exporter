//revive:disable:var-naming
package statistical_classifier

//revive:enable:var-naming

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHistory_add(t *testing.T) {
	h := newHistory(1)
	data := "test-classifications"
	expectedData := "test-classifications"

	h.add(data)
	historyData := h.list.Front().Value
	assert.EqualValues(t, expectedData, historyData)
}

type testForgetOld struct {
	historySize  int
	inputData    []string
	expectedData []string
}

func TestHistory_ForgetOld(t *testing.T) {
	testCases := []testForgetOld{
		{
			historySize:  10,
			inputData:    []string{"1", "2", "3"},
			expectedData: []string{"1", "2", "3"},
		},
		{
			historySize:  0,
			inputData:    []string{"1", "2", "3"},
			expectedData: []string{},
		},
		{
			historySize:  2,
			inputData:    []string{"1", "2", "3"},
			expectedData: []string{"2", "3"},
		},
	}

	for _, testCase := range testCases {
		testedHistory := newHistory(testCase.historySize)
		for _, item := range testCase.inputData {
			testedHistory.add(item)
		}
		var historyItems []string
		for item := range testedHistory.streamList() {
			historyItems = append(historyItems, item.(string))
		}
		assert.ElementsMatch(t, testCase.expectedData, historyItems)
	}

}

func TestStreamList(t *testing.T) {
	data := []string{"1", "2", "3"}
	h := newHistory(10)
	for _, v := range data {
		h.add(v)
	}

	var actualData []string
	for record := range h.streamList() {
		actualData = append([]string{record.(string)}, actualData...)
	}

	assert.EqualValues(t, data, actualData)
}
