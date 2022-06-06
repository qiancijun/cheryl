package balancer

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetBalancerType(t *testing.T) {
	typies := GetBalancerType()
	expect := map[string]bool{
		"round-robin": true,
		"consistence-hash": true,
	}
	assert.NotZero(t, len(typies))
	for _, v := range typies {
		t.Log(v)
		assert.Equal(t, expect[v], true)
	}
}