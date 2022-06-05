package reverseproxy

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync"
	"time"

	jsoniter "github.com/json-iterator/go"
	"github.com/qiancijun/cheryl/balancer"
	"github.com/qiancijun/cheryl/config"
	"github.com/qiancijun/cheryl/logger"
	"github.com/qiancijun/cheryl/utils"
)

type ProxyMap struct {
	sync.RWMutex
	Relations map[string]*HTTPProxy `json:"-"`
	Locations map[string]config.Location
	Limiters  map[string][]LimiterInfo
	Infos     Info
}

type Info struct {
	RouterType string
}

type LimiterInfo struct {
	Prefix      string `json:"prefix"`
	PathName    string `json:"pathName"`
	LimiterType string `json:"limiterType"`
	Volumn      int    `json:"volumn"`    // 容量
	Speed       int64  `json:"speed"`     // 速率
	MaxThread   int    `json:"maxThread"` // 最大并发数量
	Duration    int    `json:"duration"`  // 超时时间
}

func NewProxyMap() *ProxyMap {
	// router := GetRouterInstance("default").(*DefaultRouter)
	// router.acl = rt
	return &ProxyMap{
		Relations: make(map[string]*HTTPProxy),
		// Router:    router,
		Locations: make(map[string]config.Location),
		Infos: Info{
			RouterType: "default",
		},
		Limiters: make(map[string][]LimiterInfo),
	}
}

func (proxyMap *ProxyMap) Marshal() ([]byte, error) {
	proxyMap.RLock()
	defer proxyMap.RUnlock()
	res, err := jsoniter.Marshal(proxyMap)
	return res, err
}

func (proxyMap *ProxyMap) UnMarshal(serialized io.ReadCloser) error {
	if err := jsoniter.NewDecoder(serialized).Decode(&proxyMap); err != nil {
		return err
	}
	return nil
}

func (proxyMap *ProxyMap) AddRelations(pattern string, proxy *HTTPProxy, location config.Location) {
	proxy.ProxyMap = proxyMap
	proxyMap.Relations[pattern] = proxy
	proxy.ProxyMap.Locations[pattern] = location
	RouterSingleton.Add(pattern, proxy)
	proxy.HealthCheck()
}

func (proxyMap *ProxyMap) AddProxyWithLocation(l config.Location) error {
	httpProxy, err := NewHTTPProxy(l.Pattern, l.ProxyPass, balancer.Algorithm(l.BalanceMode))
	if err != nil {
		logger.Warnf("create proxy error: %s", err.Error())
		return err
	}
	proxyMap.AddRelations(l.Pattern, httpProxy, l)
	return nil
}

func (proxyMap *ProxyMap) AddProxy(pattern string, host string) error {
	proxyMap.printAllRelationKey()
	logger.Debugf("{AddHost} pattern: %s, host: %s will create proxy", pattern, host)
	httpProxy := proxyMap.Relations[pattern]
	if httpProxy == nil {
		return errors.New("pattern is not exists, please use config or webui first")
	}
	url, err := url.Parse(host)
	if err != nil {
		return err
	}
	logger.Debugf("%s will add to %s", url, pattern)
	proxy := httputil.NewSingleHostReverseProxy(url)
	originDirector := proxy.Director
	proxy.Director = func(r *http.Request) {
		originDirector(r)
		r.Header.Set(XProxy, ReverseProxy)
		r.Header.Set(XRealIP, utils.GetIP(r.RemoteAddr))
	}
	host = utils.GetHost(url)
	httpProxy.HostMap[host] = proxy
	httpProxy.Alive[host] = true
	httpProxy.HostsShutDown[host] = make(chan bool)
	httpProxy.Lb.Add(host)
	go httpProxy.healthCheck(host)
	return nil
}

func (proxyMap *ProxyMap) RemoveProxy(pattern string) error {
	proxyMap.Lock()
	defer proxyMap.Unlock()
	if _, has := proxyMap.Relations[pattern]; !has {
		return fmt.Errorf("can't find the reverseproxy with the pattern %s", pattern)
	}
	logger.Debugf("%s will remove from proxyMap", pattern)
	// TODO 超时控制
	proxyMap.Relations[pattern].ShutDown <- true
	RouterSingleton.Remove(pattern)
	delete(proxyMap.Relations, pattern)
	delete(proxyMap.Locations, pattern)
	return nil
}

func (proxyMap *ProxyMap) RemoveHost(pattern string, host string) error {
	logger.Debugf("%s will remove from the %s", host, pattern)
	httpProxy, has := proxyMap.Relations[pattern]
	for k := range httpProxy.HostsShutDown {
		logger.Debug(k)
	}
	if !has {
		return fmt.Errorf("can't find the reverseproxy with the pattern %s", pattern)
	}
	select {
	case <-time.After(5 * time.Second):
		return errors.New("shut down reverproxy timeout")
	case httpProxy.HostsShutDown[host] <- true:
		delete(httpProxy.HostMap, host)
		return nil
	}
}

func (proxyMap *ProxyMap) printAllRelationKey() {
	for k := range proxyMap.Relations {
		logger.Debug(k)
	}
}
