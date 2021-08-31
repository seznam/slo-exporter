package event

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func AssertRawEventsEqual(t *testing.T, e1, e2 Raw, checkIds bool) {
	if !checkIds && (e1 != nil && e2 != nil) {
		e1.SetId("")
		e2.SetId("")
	}
	assert.Equal(t, e1, e2)
}

