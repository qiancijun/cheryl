package ratelimit

import (
	"errors"
	"time"
)

type LimiterType string

/**
限流器在整个程序内部中会被频繁创建，由此采用工厂设计模式
*/
type RateLimiter interface {
	Take() error
	TakeWithTimeout(time.Duration) error
	SetRate(int, int64) // 设置速率
	GetVolumn() int
	GetSpeed() int64
	GetTimeout() time.Duration
	SetTimeout(time.Duration)
}

type RateLimiterFactory func() RateLimiter

var (
	LimiterAlreadyExists         = errors.New("limiter already exists")
	LimiterTypeNotSupportedError = errors.New("limiter type not supported")
	NoReaminTokenError           = errors.New("The token has been used up")
	rateLimiterFactories         = make(map[LimiterType]RateLimiterFactory)
)

func Build(t LimiterType) (RateLimiter, error) {
	factory, has := rateLimiterFactories[t]
	if !has {
		return nil, LimiterTypeNotSupportedError
	}
	return factory(), nil
}

func GetLimiterType() (res []string) {
	for k := range rateLimiterFactories {
		res = append(res,string(k))
	}
	return
}