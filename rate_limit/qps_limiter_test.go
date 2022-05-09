package ratelimit

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestQpsLimiter(t *testing.T) {
	limiter, err := Build("qps")
	assert.NoError(t, err)
	assert.NotNil(t, limiter)
	limiter.SetRate(1, 1)
	volumn, speed := limiter.GetVolumn(), limiter.GetSpeed()
	assert.Equal(t, 1, volumn)
	assert.Equal(t, int64(1), speed)
	idx := 1
	for {
		err := limiter.Take()
		assert.NoError(t, err)
		if idx == 10 {
			break
		}
		idx++
		time.Sleep(1000 * time.Millisecond)
	}
}

func TestQpsLimiterUseUp(t *testing.T) {
	limiter, err := Build("qps")
	assert.NoError(t, err)
	assert.NotNil(t, limiter)
	limiter.SetRate(2, 1)
	volumn, speed := limiter.GetVolumn(), limiter.GetSpeed()
	assert.Equal(t, 2, volumn)
	assert.Equal(t, int64(1), speed)
	idx := 1
	for {
		err := limiter.Take()
		if idx != 4 {
			assert.NoError(t, err)
		} else if idx == 4 {
			assert.Error(t, err)
			break
		}
		idx++
		time.Sleep(500 * time.Millisecond)
	}
}

func TestQpsLimiterTimeout(t *testing.T) {
	limiter, err := Build("qps")
	assert.NoError(t, err)
	assert.NotNil(t, limiter)
	limiter.SetRate(0, 1)
	err = limiter.TakeWithTimeout(500 * time.Millisecond)
	assert.Error(t, err)
}