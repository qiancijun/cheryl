package ratelimit

import (
	"context"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

type QpsRateLimiter struct {
	sync.RWMutex
	limiter *rate.Limiter
	timeout time.Duration
}

func init() {
	rateLimiterFactories["qps"] = NewQpsRateLimiter
}

func NewQpsRateLimiter() RateLimiter {
	return &QpsRateLimiter{
		limiter: rate.NewLimiter(rate.Inf, 0),
		// limiter: rate.NewLimiter(2, 1),
		timeout: -1,
	}
}

func (r *QpsRateLimiter) Take() error {
	// rate包内部已经做好了并发控制，这里不添加锁了
	if !r.limiter.Allow() {
		return NoReaminTokenError
	}
	return nil
}

func (r *QpsRateLimiter) TakeWithTimeout(timeout time.Duration) error {
	ctx, close := context.WithTimeout(context.Background(), timeout)
	defer close()
	err := r.limiter.Wait(ctx)
	if err != nil {
		return NoReaminTokenError
	}
	return nil
}

// init：初始化的个数
// speed：一秒生成多少个 token
func (r *QpsRateLimiter) SetRate(init int, speed int64) {
	r.limiter = rate.NewLimiter(rate.Every(time.Second / time.Duration(speed)), init)
}

func (r *QpsRateLimiter) GetVolumn() int {
	return r.limiter.Burst()
}

func (r *QpsRateLimiter) GetSpeed() int64 {
	return int64(float64(r.limiter.Limit()))
}


// -1代表没有配置超时
func (r *QpsRateLimiter) GetTimeout() time.Duration {
	r.RLock()
	defer r.RUnlock()
	return r.timeout
}

func (r *QpsRateLimiter) SetTimeout(time time.Duration) {
	r.Lock()
	defer r.Unlock()
	r.timeout = time
}