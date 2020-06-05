package stringmap

import (
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/pkg/labels"
	"sort"
	"strings"
)

func NewFromMetric(labels model.Metric) StringMap {
	newStringMap := StringMap{}
	for name, value := range labels {
		newStringMap[string(name)] = string(value)
	}
	return newStringMap
}

func NewFromLabels(labels labels.Labels) StringMap {
	newStringMap := StringMap{}
	for _, label := range labels {
		newStringMap[label.Name] = label.Value
	}
	return newStringMap
}

type StringMap map[string]string

// Copy returns new StringMap as a copy of the original.
func (m StringMap) Copy() StringMap {
	copied := StringMap{}
	for k, v := range m {
		copied[k] = v
	}
	return copied
}

// Merge returns new StringMap from the original one with all values from the other merged in. The other StringMap overrides original StringMap keys.
func (m StringMap) Merge(other StringMap) StringMap {
	if m == nil {
		return other
	}
	merged := m.Copy()
	for k, v := range other {
		merged[k] = v
	}
	return merged
}

// NewWith returns new StringMap with the new key and value provided.
func (m StringMap) NewWith(key, value string) StringMap {
	newMetadata := m.Copy()
	newMetadata[key] = value
	return newMetadata
}

// Keys returns non-ordered list of StringMap keys.
func (m StringMap) Keys() []string {
	var keys []string
	for k, _ := range m {
		keys = append(keys, k)
	}
	return keys
}

// AddKeys adds provided keys to the stringmap (using empty string as a value).
func (m StringMap) AddKeys(keys ...string) {
	if keys == nil {
		return
	}
	for _, k := range keys {
		m[k] = ""
	}
}

// Keys returns sorted list of StringMap keys.
func (m StringMap) SortedKeys() []string {
	keys := m.Keys()
	sort.Strings(keys)
	return keys
}

// Values returns non-ordered list of StringMap values.
func (m StringMap) Values() []string {
	var values []string
	for _, v := range m {
		values = append(values, v)
	}
	return values
}

// ValuesByKeys returns list of corresponding StringMap values for the given keys in appropriate order.
func (m StringMap) ValuesByKeys(keys []string) []string {
	var values []string
	for _, key := range keys {
		val, ok := m[key]
		if ok {
			values = append(values, val)
		}
	}
	return values
}

// Matches returns true if every key of the original StringMap is present in the other StringMap and refers to the same value, otherwise returns false.
func (m StringMap) Matches(other StringMap) bool {
	if len(m) > len(other) {
		return false
	}
	for k, v := range m {
		otherV, ok := other[k]
		if !ok {
			return false
		}
		if v != otherV {
			return false
		}
	}
	return true
}

//String returns ordered key-value list separated with comma.
func (m StringMap) String() string {
	items := make([]string, len(m))
	for i, key := range m.SortedKeys() {
		val, ok := m[key]
		if !ok {
			continue
		}
		items[i] = key + `="` + val + `"`
	}
	return strings.Join(items, ",")
}

// Lowercase creates new StringMap with lowercase keys and values.
func (m StringMap) Lowercase() StringMap {
	lower := StringMap{}
	for k, v := range m {
		lower[strings.ToLower(k)] = strings.ToLower(v)
	}
	return lower
}

// Select returns new StringMap with selected keys from the original StringMap if found.
func (m StringMap) Select(keys []string) StringMap {
	selected := StringMap{}
	for _, key := range keys {
		val, ok := m[key]
		if ok {
			selected[key] = val
		}
	}
	return selected
}

// Without returns new StringMap with without specified keys from the original StringMap.
func (m StringMap) Without(keys []string) StringMap {
	if m == nil {
		return nil
	}
	if len(keys) == 0 {
		return m
	}
	other := m.Copy()
	for _, key := range keys {
		if _, ok := other[key]; ok {
			delete(other, key)
		}
	}
	return other
}

// AsPrometheusLabels converts the stringmap to prometheus labels as is.
func (m StringMap) AsPrometheusLabels() labels.Labels {
	newLabels := make(labels.Labels, len(m))
	i := 0
	for k, v := range m {
		newLabels[i] = labels.Label{Name: k, Value: v}
		i++
	}
	return newLabels
}

