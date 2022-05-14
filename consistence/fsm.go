package consistence

import (
	"errors"
	"fmt"
	"io"
	"log"

	"com.cheryl/cheryl/balancer"
	"com.cheryl/cheryl/config"
	"com.cheryl/cheryl/logger"
	reverseproxy "com.cheryl/cheryl/reverse_proxy"
	"github.com/hashicorp/raft"
	jsoniter "github.com/json-iterator/go"
)

var (
	HttpProxyNotExistsError = errors.New("HttpProxy not exists")
)

type FSM struct {
	ctx *StateContext
	log *log.Logger
}

/**
opt: 操作类型：

*/
type LogEntryData struct {
	Opt         int
	Key         string
	Value       map[string]string
	Location    config.Location
	LimiterInfo reverseproxy.LimiterInfo
}

// Apply applies a Raft log entry to the key-value store.
func (f *FSM) Apply(logEntry *raft.Log) interface{} {
	e := LogEntryData{}

	if err := jsoniter.Unmarshal(logEntry.Data, &e); err != nil {
		errMsg := fmt.Sprintf("Failed unmarshaling Raft log entry. This is a bug. %s", err.Error())
		logger.Warn(errMsg)
		return err
	}
	var ret interface{}
	opt := e.Opt
	logger.Debugf("FSM has received logEntry, optType: %d", opt)
	switch opt {
	case 1:
		ret = f.doNewHttpProxy(e)
	case 2:
		ret = f.doSetRateLimiter(e)
	}
	return ret
}

func (f *FSM) Snapshot() (raft.FSMSnapshot, error) {
	return &snapshot{proxyMap: f.ctx.State.ProxyMap}, nil
}

func (f *FSM) Restore(serialized io.ReadCloser) error {
	err := f.ctx.State.ProxyMap.UnMarshal(serialized)
	if err != nil {
		logger.Errorf("can't restore State: %s", err.Error())
		return err
	}
	
	router := f.ctx.State.ProxyMap.Router
	for _, l := range f.ctx.State.ProxyMap.Locations {
		httpProxy, err := reverseproxy.NewHTTPProxy(l.Pattern, l.ProxyPass, balancer.Algorithm(l.BalanceMode))
		if err != nil {
			logger.Errorf("create proxy error: %s", err)
			return err
		}
		logger.Debugf("{doNewHttpProxy} add new httpProxy %s", l.Pattern)
		router.Add(l.Pattern, httpProxy)
		httpProxy.HealthCheck()
	}
	for key, limiters := range f.ctx.State.ProxyMap.Limiters {
		httpProxy, has := f.ctx.State.ProxyMap.Relations[key]
		if !has { continue }
		for _, limiter := range limiters {
			router.SetRateLimiter(httpProxy, limiter)
		}
	}
	return nil
}

func (f *FSM) doNewHttpProxy(logEntry LogEntryData) error {
	f.ctx.State.ProxyMap.Lock()
	defer f.ctx.State.ProxyMap.Unlock()
	key, l := logEntry.Key, logEntry.Location

	if _, ok := f.ctx.State.ProxyMap.Relations[key]; ok {
		logger.Debugf("{doNewHttpProxy} %s already exists in relations", key)
		return nil
	}
	logger.Debugf("{doNewHttpProxy} receive new Log: %s, %s", key, l)
	httpProxy, err := reverseproxy.NewHTTPProxy(l.Pattern, l.ProxyPass, balancer.Algorithm(l.BalanceMode))
	if err != nil {
		logger.Errorf("create proxy error: %s", err)
		return err
	}
	httpProxy.ProxyMap = f.ctx.State.ProxyMap
	f.ctx.State.ProxyMap.Relations[l.Pattern] = httpProxy
	f.ctx.State.ProxyMap.Locations[l.Pattern] = &l
	logger.Debugf("{doNewHttpProxy} add new httpProxy %s", key)
	f.ctx.State.ProxyMap.Router.Add(key, httpProxy)
	httpProxy.HealthCheck()
	return nil
}

func (f *FSM) doSetRateLimiter(logEntry LogEntryData) error {
	f.ctx.State.ProxyMap.Lock()
	defer f.ctx.State.ProxyMap.Unlock()
	key, limiterInfo := logEntry.Key, logEntry.LimiterInfo
	router := f.ctx.State.ProxyMap.Router
	httpProxy, has := f.ctx.State.ProxyMap.Relations[key]
	if !has {
		return HttpProxyNotExistsError
	}
	return router.SetRateLimiter(httpProxy, limiterInfo)
}