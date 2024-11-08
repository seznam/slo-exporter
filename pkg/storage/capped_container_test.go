package storage

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_Container_Capacity(t *testing.T) {
	tests := []struct {
		name     string
		capacity int
	}{
		{name: "check capacity", capacity: 0},
		{name: "check capacity", capacity: 3},
		{name: "check capacity", capacity: 100},
	}
	for _, tt := range tests {
		containers := []CappedContainer{
			NewInMemoryCappedContainer(tt.capacity),
		}
		for _, c := range containers {
			t.Run(fmt.Sprintf(" | %s | %s of %d ", reflect.TypeOf(c).String(), tt.name, tt.capacity), func(t *testing.T) {
				assert.Equal(t, c.Capacity(), tt.capacity, fmt.Sprintf("Expected container capacity: %d, but got: %d", tt.capacity, c.Len()))
			})
		}
	}
}

func Test_CappedContainer_Capping(t *testing.T) {
	tests := []struct {
		name          string
		capacity      int
		itemsToAdd    []interface{}
		expectedItems []interface{}
	}{
		{name: "container with negative capacity", capacity: -1, itemsToAdd: []interface{}{1, 2, 3}, expectedItems: []interface{}{}},
		{name: "container with zero capacity", capacity: 0, itemsToAdd: []interface{}{1, 2, 3}, expectedItems: []interface{}{}},
		{name: "container with no capacity limit", capacity: 100, itemsToAdd: []interface{}{1, 2, 3}, expectedItems: []interface{}{1, 2, 3}},
		{name: "container with limited capacity", capacity: 3, itemsToAdd: []interface{}{1, 2, 3, 4, 5}, expectedItems: []interface{}{3, 4, 5}},
	}
	for _, tt := range tests {
		containers := []Container{
			NewInMemoryCappedContainer(tt.capacity),
		}
		for _, c := range containers {
			t.Run(fmt.Sprintf(" | %s | %s", reflect.TypeOf(c).String(), tt.name), func(t *testing.T) {
				for _, item := range tt.itemsToAdd {
					c.Add(item)
				}
				var streamedItems []interface{}
				for i := range c.Stream() {
					streamedItems = append(streamedItems, i)
				}
				assert.ElementsMatch(t, streamedItems, tt.expectedItems, fmt.Sprintf("Expected streamed items: %s, but got: %s", tt.expectedItems, streamedItems))
			})
		}
	}
}
