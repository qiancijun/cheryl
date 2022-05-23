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
	Concurrent chan bool
	Volumn int
	Timeout time.Duration
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
	return limiter.Volumn
}

func (limiter *ConcurrentLimit) GetTimeout() time.Duration {
	limiter.RLock()
	defer limiter.RUnlock()
	return limiter.Timeout
}

func (limiter *ConcurrentLimit) SetTimeout(timeout time.Duration) {
	limiter.Lock()
	defer limiter.Unlock()
	limiter.Timeout = timeout
}

func (limiter *ConcurrentLimit) SetRate(_ int, _ int64) {}
func (limiter *ConcurrentLimit) GetSpeed() int64 { return -1 }