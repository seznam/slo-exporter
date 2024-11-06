package storage

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_Container_Add(t *testing.T) {
	tests := []struct {
		name string
		item interface{}
	}{
		{name: "add number", item: 1},
		{name: "add string", item: "foo"},
		{name: "add struct", item: struct{}{}},
	}
	for _, tt := range tests {
		containerCapacity := 1
		containers := []Container{
			NewInMemoryCappedContainer(containerCapacity),
		}
		for _, c := range containers {
			t.Run(fmt.Sprintf(" | %s | %s", reflect.TypeOf(c).String(), tt.name), func(_ *testing.T) {
				c.Add(tt.item)
			})
		}
	}
}

func Test_Container_Len(t *testing.T) {
	tests := []struct {
		name          string
		numberOfItems int
	}{
		{name: "check length", numberOfItems: 0},
		{name: "check length", numberOfItems: 3},
		{name: "check length", numberOfItems: 100},
	}
	for _, tt := range tests {
		containers := []Container{
			NewInMemoryCappedContainer(tt.numberOfItems),
		}
		for _, c := range containers {
			t.Run(fmt.Sprintf(" | %s | %s with %d items", reflect.TypeOf(c).String(), tt.name, tt.numberOfItems), func(t *testing.T) {
				for i := 0; i < tt.numberOfItems; i++ {
					c.Add(struct{}{})
				}
				assert.Equal(t, c.Len(), tt.numberOfItems, fmt.Sprintf("Expected container length: %d, but got: %d", tt.numberOfItems, c.Len()))
			})
		}
	}
}

func Test_Container_Stream(t *testing.T) {
	tests := []struct {
		name  string
		items []interface{}
	}{
		{name: "stream numbers", items: []interface{}{1, 2, 3}},
		{name: "stream strings", items: []interface{}{"a", "b", "c"}},
		{name: "stream structs", items: []interface{}{struct{}{}, struct{}{}, struct{}{}}},
	}
	for _, tt := range tests {
		containers := []Container{
			NewInMemoryCappedContainer(len(tt.items)),
		}
		for _, c := range containers {
			t.Run(fmt.Sprintf(" | %s | %s", reflect.TypeOf(c).String(), tt.name), func(t *testing.T) {
				for _, item := range tt.items {
					c.Add(item)
				}
				assert.Equal(t, c.Len(), len(tt.items))
				var streamedItems []interface{}
				for i := range c.Stream() {
					streamedItems = append(streamedItems, i)
				}
				assert.ElementsMatch(t, streamedItems, tt.items, fmt.Sprintf("Expected streamed items: %s, but got: %s", tt.items, streamedItems))
			})
		}
	}
}
