package stringmap

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

type stringMapMatchTestCase struct {
	a      StringMap
	b      StringMap
	result bool
}

func TestStringMap_Matches(t *testing.T) {
	testCases := []stringMapMatchTestCase{
		{a: StringMap{"a": "1"}, b: StringMap{"b": "2"}, result: false},
		{a: StringMap{"a": "1"}, b: StringMap{"a": "2"}, result: false},
		{a: StringMap{"a": "1"}, b: StringMap{"a": "1"}, result: true},
		{a: StringMap{}, b: StringMap{"a": "1"}, result: true},
		{a: StringMap{"a": "1"}, b: StringMap{}, result: false},
	}

	for _, tc := range testCases {
		assert.Equal(t, tc.result, tc.a.Matches(tc.b))
	}
}

type stringMapMergeTestCase struct {
	a      StringMap
	b      StringMap
	result StringMap
}

func TestStringMap_Merge(t *testing.T) {
	testCases := []stringMapMergeTestCase{
		{a: StringMap{"a": "1"}, b: StringMap{"b": "2"}, result: StringMap{"a": "1", "b": "2"}},
		{a: StringMap{"a": "1"}, b: StringMap{"a": "2"}, result: StringMap{"a": "2"}},
		{a: StringMap{"a": "1"}, b: StringMap{}, result: StringMap{"a": "1"}},
		{a: StringMap{}, b: StringMap{"a": "1"}, result: StringMap{"a": "1"}},
		{a: StringMap{}, b: StringMap{}, result: StringMap{}},
		{a: nil, b: StringMap{"a": "1"}, result: StringMap{"a": "1"}},
		{a: StringMap{"a": "1"}, b: nil, result: StringMap{"a": "1"}},
	}

	for _, tc := range testCases {
		assert.Equal(t, tc.result, tc.a.Merge(tc.b))
	}
}

type stringMapStringsTestCase struct {
	meta StringMap
	res  []string
}

func TestStringMap_Keys(t *testing.T) {
	testCases := []stringMapStringsTestCase{
		{meta: StringMap{"a": "1"}, res: []string{"a"}},
		{meta: StringMap{"a": "1", "b": "2"}, res: []string{"a", "b"}},
		{meta: StringMap{}, res: []string{}},
	}

	for _, tc := range testCases {
		assert.ElementsMatch(t, tc.res, tc.meta.Keys())
	}
}

func TestStringMap_Values(t *testing.T) {
	testCases := []stringMapStringsTestCase{
		{meta: StringMap{"a": "1"}, res: []string{"1"}},
		{meta: StringMap{"a": "1", "b": "2"}, res: []string{"1", "2"}},
		{meta: StringMap{}, res: []string{}},
	}

	for _, tc := range testCases {
		assert.ElementsMatch(t, tc.res, tc.meta.Values())
	}
}

type stringMapStringTestCase struct {
	meta StringMap
	res  string
}

func TestStringMap_String(t *testing.T) {
	testCases := []stringMapStringTestCase{
		{meta: StringMap{"a": "1"}, res: `a="1"`},
		{meta: StringMap{"a": "1", "b": "2"}, res: `a="1",b="2"`},
		{meta: StringMap{"b": "1", "a": "2"}, res: `a="2",b="1"`},
		{meta: StringMap{"": ""}, res: ``},
		{meta: StringMap{"a": ""}, res: `a=""`},
		{meta: StringMap{}, res: ``},
	}

	for _, tc := range testCases {
		assert.Equal(t, tc.res, tc.meta.String())
	}
}

type stringMapSelectTestCase struct {
	meta StringMap
	keys []string
	res  StringMap
}

func TestStringMap_Select(t *testing.T) {
	testCases := []stringMapSelectTestCase{
		{meta: StringMap{"a": "1"}, keys: []string{}, res: StringMap{}},
		{meta: StringMap{"a": "1"}, keys: []string{"a"}, res: StringMap{"a": "1"}},
		{meta: StringMap{"a": "1"}, keys: []string{"b"}, res: StringMap{}},
		{meta: StringMap{}, keys: []string{"b"}, res: StringMap{}},
		{meta: StringMap{"a": "1", "b": "2"}, keys: []string{"a", "b"}, res: StringMap{"a": "1", "b": "2"}},
	}

	for _, tc := range testCases {
		assert.Equal(t, tc.res, tc.meta.Select(tc.keys))
	}
}

type stringMapLowercaseTestCase struct {
	meta StringMap
	res  StringMap
}

func TestStringMap_Lowercase(t *testing.T) {
	testCases := []stringMapLowercaseTestCase{
		{meta: StringMap{"A": "1"}, res: StringMap{"a": "1"}},
		{meta: StringMap{"AbfE": "s2EEr"}, res: StringMap{"abfe": "s2eer"}},
	}

	for _, tc := range testCases {
		assert.Equal(t, tc.res, tc.meta.Lowercase())
	}
}


type stringMapWithoutTestCase struct {
	a      StringMap
	b      []string
	result StringMap
}

func TestStringMap_Without(t *testing.T) {
	testCases := []stringMapWithoutTestCase{
		{a: StringMap{"a": "1", "b": "2"}, b: []string{"b"}, result: StringMap{"a": "1"}},
		{a: StringMap{"a": "2"}, b: []string{"a"}, result: StringMap{}},
		{a: StringMap{"a": "1"}, b: []string{}, result: StringMap{"a": "1"}},
		{a: StringMap{}, b: []string{}, result: StringMap{}},
		{a: nil, b: []string{"A"}, result: nil},
		{a: StringMap{"a": "1"}, b: nil, result: StringMap{"a": "1"}},
	}

	for _, tc := range testCases {
		assert.Equal(t, tc.result, tc.a.Without(tc.b), tc)
	}
}