package ratelimit

import (
	"testing"

	"golang.org/x/time/rate"
)

func TestRateLimiter(t *testing.T) {
	v := rate.Limit(1 / 100)
	t.Log(v)
}
