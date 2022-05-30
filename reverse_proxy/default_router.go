package reverseproxy

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"com.cheryl/cheryl/acl"
	"com.cheryl/cheryl/logger"
	ratelimit "com.cheryl/cheryl/rate_limit"
	"com.cheryl/cheryl/utils"
)

// 使用哈希记录路由前缀
type DefaultRouter struct {
	sync.RWMutex
	hosts map[string]*HTTPProxy
}

func (r *DefaultRouter) Add(p string, proxy *HTTPProxy) {
	r.Lock()
	defer r.Unlock()
	r.hosts[p] = proxy
}

func (r *DefaultRouter) Remove(p string) {
	r.Lock()
	defer r.Unlock()
	delete(r.hosts, p)
}

func (r *DefaultRouter) HasPrefix(p string) bool {
	r.RLock()
	defer r.RUnlock()
	_, has := r.hosts[p]
	return has
}

// 根据请求的路径分割前缀，找最长匹配的路由
func (r *DefaultRouter) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	logger.Infof("%s can't catch any path", req.URL)
	// accessControlList
	isPermit := r.accessControl(req)
	if !isPermit {
		w.WriteHeader(403)
		return
	}

	// route
	httpProxy, Realpath := r.Route(w, req)
	if httpProxy == nil {
		w.WriteHeader(404)
		return
	}

	// LoadBalance
	host, err := httpProxy.Lb.Balance(utils.GetIP(req.RemoteAddr))
	if err != nil {
		w.WriteHeader(http.StatusBadGateway)
		errMsg := fmt.Sprintf("balancer error: %s", err.Error())
		w.Write([]byte(errMsg))
		return
	}
	httpProxy.Lb.Inc(host)
	defer httpProxy.Lb.Done(host)

	// Rate Limit
	err = r.invaildToken(httpProxy, Realpath)
	if err != nil {
		errMsg := fmt.Sprintf("route error: %s", err.Error())
		logger.Debug(errMsg)
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(errMsg))
		return
	}

	// redirect
	req.URL.Path = Realpath
	httpProxy.HostMap[host].ServeHTTP(w, req)
}

// 具体路由选择的算法
func (r *DefaultRouter) Route(w http.ResponseWriter, req *http.Request) (*HTTPProxy, string) {

	path := req.URL.Path
	nextPath := path
	// O(n) 倒叙搜索
	var httpProxy *HTTPProxy
	var i int
	for i = len(path) - 1; i >= 0; i-- {
		if path[i] == '/' {
			nextPath = path[:i]
			logger.Debugf("debug: %s try to catch path", nextPath)
			// 找到了最长匹配的前缀路由，负载均衡转发请求
			if r.HasPrefix(nextPath) {
				httpProxy = r.hosts[nextPath]
				logger.Debugf("debug: DefaultRouter has found the longest path: %s", nextPath)

				// 将前缀覆盖重写
				path = path[i:]
				logger.Debugf("rewrite the path: %s", path)
				return httpProxy, path
			}
		}
	}
	// 没有找到匹配的路由，返回404
	logger.Debugf("can't find any path can accord with: %s", path)
	return nil, ""
}

// 将此次接口访问记录到反向代理器中，用于配置限流
func (r *DefaultRouter) ConfigRate(httpProxy *HTTPProxy, path string) error {
	if httpProxy == nil {
		errMsg := "default_router can't find the httpProxy"
		return errors.New(errMsg)
	}
	methods := httpProxy.Methods
	_, has := methods[path]
	if !has {
		r.Lock()
		defer r.Unlock()
		// 默认创建一个 qps 限流器
		limiter, err := ratelimit.Build("qps")
		if err != nil {
			errMsg := "can't find qps limiter"
			return errors.New(errMsg)
		}
		methods[path] = limiter
		// 在 ProxyMap 中记录
		proxyMap := httpProxy.ProxyMap
		limiters := proxyMap.Limiters
		limiters[httpProxy.Pattern] = append(limiters[httpProxy.Pattern], LimiterInfo{
			PathName:    path,
			LimiterType: "qps",
			Volumn:      -1,
			Speed:       0,
			MaxThread:   -1,
			Duration:    0,
		})

	} else {
		return ratelimit.LimiterAlreadyExists
	}
	return nil
}

func (r *DefaultRouter) invaildToken(proxy *HTTPProxy, api string) error {
	methods := proxy.Methods
	limiter := methods[api]
	// 还未配置限流器，经过第一次访问之后，默认创建一个 qps 限流器
	if limiter == nil {
		err := r.ConfigRate(proxy, api)
		return err
	}
	logger.Debugf("{invaildToken} method: %s 's limiter info: volumn: %d speed: %d", api, limiter.GetVolumn(), limiter.GetVolumn())
	timeout := limiter.GetTimeout()

	var err error
	if timeout == 0 {
		err = limiter.Take()
	} else {
		err = limiter.TakeWithTimeout(timeout)
	}
	return err
}

func (r *DefaultRouter) SetRateLimiter(httpProxy *HTTPProxy, info LimiterInfo) error {
	logger.Debugf("{SetRateLimiter} pathName: %s limiterType: %s volumn: %d speed: %d maxThread: %d", info.PathName, info.LimiterType, info.Volumn, info.Speed, info.MaxThread)
	methods := httpProxy.Methods
	r.Lock()
	defer r.Unlock()

	limiter, err := ratelimit.Build(ratelimit.LimiterType(info.LimiterType))
	if err != nil {
		return err
	}
	if info.Speed != 0 {
		limiter.SetRate(info.Volumn, info.Speed)
	}
	if info.Duration != 0 {
		limiter.SetTimeout(time.Duration(info.Duration) * time.Millisecond)
	}
	methods[info.PathName] = limiter
	// 在 ProxyMap 中记录
	limiters := httpProxy.ProxyMap.Limiters
	limiters[httpProxy.Pattern] = append(limiters[httpProxy.Pattern], info)
	return nil
}

func (r *DefaultRouter) accessControl(req *http.Request) bool {
	ipWithPort := strings.Split(utils.RemoteIp(req), ":")
	ip := ipWithPort[0]
	logger.Debugf("%s will access the system", ip)
	radixTree := acl.AccessControlList
	ret := radixTree.Search(ip) == ""
	if ret {
		logger.Debugf("%s is forbidden to access system")
	}
	return ret
}
