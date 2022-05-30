package consistence

import (
	"errors"
	"fmt"
	"io"
	"log"

	"com.cheryl/cheryl/acl"
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
	Value       string
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
	case 3:
		ret = f.doHandleAcl(e)
	}
	return ret
}

func (f *FSM) Snapshot() (raft.FSMSnapshot, error) {
	return &snapshot{
		ProxyMap: f.ctx.State.ProxyMap, 
		RadixTree: acl.AccessControlList,
	}, nil
}

func (f *FSM) Restore(serialized io.ReadCloser) error {
	logger.Debug("found snapshot file, ready to restore")

	var s snapshot
	err := s.UnMarshal(serialized)
	if err != nil {
		logger.Errorf("can't restore State: %s", err.Error())
		return err
	}
	// 重新创建映射关系
	f.ctx.State.ProxyMap = s.ProxyMap
	f.ctx.State.ProxyMap.Relations = make(map[string]*reverseproxy.HTTPProxy)
	// 重新创建 Router
	routerType := f.ctx.State.ProxyMap.Infos.RouterType
	reverseproxy.GetRouterInstance(routerType)
	// f.ctx.State.ProxyMap.Router = router
	logger.Debugf("{Restore} locations: %s", f.ctx.State.ProxyMap.Locations)
	for _, l := range f.ctx.State.ProxyMap.Locations {
		logger.Debugf("{Restore} found location: pattern: %s proxypass: %s balanceMode: %s", l.Pattern, l.ProxyPass, l.BalanceMode)
		httpProxy, err := reverseproxy.NewHTTPProxy(l.Pattern, l.ProxyPass, balancer.Algorithm(l.BalanceMode))
		if err != nil {
			logger.Errorf("create proxy error: %s", err)
			return err
		}
		logger.Debugf("{doNewHttpProxy} add new httpProxy %s", l.Pattern)
		
		f.ctx.State.ProxyMap.AddRelations(l.Pattern, httpProxy, l)
	}

	for key, limiters := range f.ctx.State.ProxyMap.Limiters {
		httpProxy, has := f.ctx.State.ProxyMap.Relations[key]
		if !has { continue }
		for _, limiter := range limiters {
			// add methods to ProxyMap, then set rate limiter
			httpProxy.SetRateLimiter(limiter)
		}
	}

	// 重新构建 RadixTree
	acl.AccessControlList = acl.NewRadixTree()
	for key := range s.RadixTree.Record {
		logger.Debugf("{Restore} acl key: %s", key)
		acl.AccessControlList.Add(key, key)
	}
	return nil
}
// func (f *FSM) Restore(serialized io.ReadCloser) error {
// 	logger.Debug("found snapshot file, ready to restore")

// 	var s *snapshot
// 	err := s.UnMarshal(serialized)
// 	if err != nil {
// 		logger.Errorf("can't restore State: %s", err.Error())
// 		return err
// 	}
// 	proxyMap := s.ProxyMap
// 	rt := s.RadixTree

// 	f.ctx.State.ProxyMap.Relations = make(map[string]*reverseproxy.HTTPProxy)
// 	// err := f.ctx.State.ProxyMap.UnMarshal(serialized)
// 	// if err != nil {
// 	// 	logger.Errorf("can't restore State: %s", err.Error())
// 	// 	return err
// 	}

// 	router := f.ctx.State.ProxyMap.Router
// 	logger.Debugf("{Restore} locations: %s", f.ctx.State.ProxyMap.Locations)
// 	for _, l := range f.ctx.State.ProxyMap.Locations {
// 		logger.Debugf("{Restore} found location: pattern: %s proxypass: %s balanceMode: %s", l.Pattern, l.ProxyPass, l.BalanceMode)
// 		httpProxy, err := reverseproxy.NewHTTPProxy(l.Pattern, l.ProxyPass, balancer.Algorithm(l.BalanceMode))
// 		if err != nil {
// 			logger.Errorf("create proxy error: %s", err)
// 			return err
// 		}
// 		logger.Debugf("{doNewHttpProxy} add new httpProxy %s", l.Pattern)
		
// 		f.ctx.State.ProxyMap.AddRelations(l.Pattern, httpProxy, l)
// 	}

// 	for key, limiters := range f.ctx.State.ProxyMap.Limiters {
// 		httpProxy, has := f.ctx.State.ProxyMap.Relations[key]
// 		if !has { continue }
// 		for _, limiter := range limiters {
// 			// add methods to ProxyMap, then set rate limiter
// 			router.SetRateLimiter(httpProxy, limiter)
// 		}
// 	}

// 	err = acl.AccessControlList.UnMarshal(serialized)
// 	if err != nil {
// 		logger.Errorf("can't restore AccessControlList: %s", err.Error())
// 		return err
// 	}

// 	for key := range acl.AccessControlList.Record {
// 		logger.Debugf("{Restore} acl key: %s", key)
// 		acl.AccessControlList.Add(key, key)
// 	}
// 	return nil
// }

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
	f.ctx.State.ProxyMap.AddRelations(l.Pattern, httpProxy, l)
	
	logger.Debugf("{doNewHttpProxy} add new httpProxy %s", key)
	return nil
}

func (f *FSM) doSetRateLimiter(logEntry LogEntryData) error {
	f.ctx.State.ProxyMap.Lock()
	defer f.ctx.State.ProxyMap.Unlock()
	key, limiterInfo := logEntry.Key, logEntry.LimiterInfo
	// router := f.ctx.State.ProxyMap.Router
	httpProxy, has := f.ctx.State.ProxyMap.Relations[key]
	if !has {
		return HttpProxyNotExistsError
	}
	return httpProxy.SetRateLimiter(limiterInfo)
}

func (f *FSM) doHandleAcl(logEntry LogEntryData) error {
	ipNet, optType := logEntry.Key, logEntry.Value
	if optType == "delete" {
		err := acl.AccessControlList.Delete(ipNet)
		if err != nil {
			return err
		}
	} else if optType == "insert" {
		err := acl.AccessControlList.Add(ipNet, ipNet)
		return err
	}
	return nil
}