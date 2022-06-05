package cheryl

import (
	"encoding/binary"
	"errors"
	"io"
	"log"

	"github.com/hashicorp/raft"
	jsoniter "github.com/json-iterator/go"
	"github.com/qiancijun/cheryl/acl"
	"github.com/qiancijun/cheryl/config"
	"github.com/qiancijun/cheryl/logger"
	reverseproxy "github.com/qiancijun/cheryl/reverse_proxy"
)

var (
	HttpProxyNotExistsError = errors.New("HttpProxy not exists")
)

type FSM struct {
	ctx *StateContext
	log *log.Logger
}

func (f *FSM) Apply(logEntry *raft.Log) interface{} {
	data := logEntry.Data
	
	optType := binary.BigEndian.Uint16(data)
	data = data[2:]	
	
	var ret interface{}
	logger.Debugf("FSM has received logEntry, optType: %d", optType)
	switch optType {
	case uint16(1):
		ret = f.doNewHttpProxy(data)
	case uint16(2):
		ret = f.doSetRateLimiter(data)
	case uint16(3):
		ret = f.doHandleAcl(data)
	case uint16(4):
		ret = f.doRemoveProxy(data)
	case uint16(5):
		ret = f.doRemoveHost(data)
	case uint16(6):
		ret = f.doAddHost(data)
	default:
		logger.Warnf("Unknown log entry type: %d", optType)
	}
	return ret
}

func (f *FSM) Snapshot() (raft.FSMSnapshot, error) {
	return &snapshot{
		ProxyMap:  f.ctx.State.ProxyMap,
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
		err := f.ctx.State.ProxyMap.AddProxyWithLocation(l)
		if err != nil {
			logger.Errorf("can't create proxy: %s", err.Error())
		}
	}

	for key, limiters := range f.ctx.State.ProxyMap.Limiters {
		httpProxy, has := f.ctx.State.ProxyMap.Relations[key]
		if !has {
			continue
		}
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

func (f *FSM) doNewHttpProxy(data []byte) error {
	f.ctx.State.ProxyMap.Lock()
	defer f.ctx.State.ProxyMap.Unlock()
	l := config.Location{}
	if err := jsoniter.Unmarshal(data, &l); err != nil {
		logger.Warnf("{doNewHttpProxy} can't resolve the data: %s", err.Error())
		return err
	}

	if _, ok := f.ctx.State.ProxyMap.Relations[l.Pattern]; ok {
		logger.Debugf("{doNewHttpProxy} %s already exists in relations", l.Pattern)
		return nil
	}
	logger.Debugf("{doNewHttpProxy} receive new Log: %s, %s", l.Pattern, l)
	err := f.ctx.State.ProxyMap.AddProxyWithLocation(l)
	if err != nil {
		logger.Warnf("create proxy error: %s", err.Error())
	}

	logger.Debugf("{doNewHttpProxy} add new httpProxy %s", l.Pattern)
	return nil
}

func (f *FSM) doSetRateLimiter(data []byte) error {
	f.ctx.State.ProxyMap.Lock()
	defer f.ctx.State.ProxyMap.Unlock()
	info := reverseproxy.LimiterInfo{}
	if err := jsoniter.Unmarshal(data, &info); err != nil {
		logger.Warnf("can't set rate limiter")
		return err
	}
	httpProxy, has := f.ctx.State.ProxyMap.Relations[info.Prefix]
	if !has {
		return HttpProxyNotExistsError
	}
	return httpProxy.SetRateLimiter(info)
}

func (f *FSM) doHandleAcl(data []byte) error {
	aclLog := AclLog{}
	if err := jsoniter.Unmarshal(data, &aclLog); err != nil {
		logger.Warnf("can't resolve aclLog")
		return err
	}
	optType, ipNet := aclLog.Pattern, aclLog.IpAddress
	if optType == 0 {
		err := acl.AccessControlList.Delete(ipNet)
		if err != nil {
			return err
		}
	} else if optType == 1 {
		err := acl.AccessControlList.Add(ipNet, ipNet)
		return err
	}
	return nil
}

func (f *FSM) doRemoveProxy(data []byte) error {
	proxy := string(data);
	return f.ctx.State.ProxyMap.RemoveProxy(proxy)
}

func (f *FSM) doRemoveHost(data []byte) error {
	removeHostLog := HostLog{}
	if err := jsoniter.Unmarshal(data, &removeHostLog); err != nil {
		logger.Warnf("can't resolve HostLog")
		return err
	}
	return f.ctx.State.ProxyMap.RemoveHost(removeHostLog.Pattern, removeHostLog.Host)
}

func (f *FSM) doAddHost(data []byte) error {
	addHostLog := HostLog{}
	if err := jsoniter.Unmarshal(data, &addHostLog); err != nil {
		logger.Warnf("can't resolve HostLog")
		return err
	}
	return f.ctx.State.ProxyMap.AddProxy(addHostLog.Pattern, addHostLog.Host)
}