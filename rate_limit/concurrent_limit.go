package ratelimit

import (
	"sync"
	"time"
)

// type RateLimiter interface {
// 	Take() error
// 	TakeWithTimeout(time.Duration) error
// 	SetRate(int, int64) // 设置速率
// 	GetVolumn() int
// 	GetSpeed() int64
// 	GetTimeout() time.Duration
// 	SetTimeout(time.Duration)
// }

type ConcurrentLimit struct {
	sync.RWMutex
	concurrent chan bool
	volumn int
	timeout time.Duration
}

func (limiter *ConcurrentLimit) Take() error {
	return nil	
}

func (limiter *ConcurrentLimit) TakeWithTimeout() error {
	return nil
}

func (limiter *ConcurrentLimit) GetVolumn() int {
	limiter.RLock()
	limiter.RUnlock()
	return limiter.volumn
}

func (limiter *ConcurrentLimit) GetTimeout() time.Duration {
	limiter.RLock()
	defer limiter.RUnlock()
	return limiter.timeout
}

func (limiter *ConcurrentLimit) SetTimeout(timeout time.Duration) {
	limiter.Lock()
	defer limiter.Unlock()
	limiter.timeout = timeout
}

func (limiter *ConcurrentLimit) SetRate(volume int, _ int64) {
	limiter.Lock()
	defer limiter.Unlock()
	limiter.concurrent = make(chan bool, volume)
}
func (limiter *ConcurrentLimit) GetSpeed() int64 { return -1 }