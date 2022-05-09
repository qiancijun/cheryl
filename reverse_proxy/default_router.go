package reverseproxy

import (
	"errors"
	"fmt"
	"net/http"
	"sync"

	"com.cheryl/cheryl/logger"
	ratelimit "com.cheryl/cheryl/rate_limit"
	"com.cheryl/cheryl/utils"
)

// 使用哈希记录路由前缀
type DefaultRouter struct {
	sync.RWMutex
	hosts     map[string]*HTTPProxy
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

func (r *DefaultRouter) HasPerfix(p string) bool {
	r.RLock()
	defer r.RUnlock()
	_, has := r.hosts[p]
	return has
}

// 根据请求的路径分割前缀，找最长匹配的路由
func (r *DefaultRouter) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	logger.Infof("%s can't catch any path", req.URL)
	r.route(w, req)
}

// 具体路由选择的算法
func (r *DefaultRouter) route(w http.ResponseWriter, req *http.Request) {
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
			if r.HasPerfix(nextPath) {
				httpProxy = r.hosts[nextPath]

				// 在进行路由转发之前，尝试从代理器中获取限流器，获取 Token 后再转发
				err := r.invaildToken(httpProxy, nextPath)
				if err != nil {
					errMsg := fmt.Sprintf("route error: %s", err.Error())
					logger.Debug(errMsg)
					w.WriteHeader(http.StatusBadRequest)
					w.Write([]byte(errMsg))
					return
				}

				host, err := httpProxy.lb.Balance(utils.GetIP(req.RemoteAddr))
				logger.Debugf("debug: DefaultRouter has found the longest path: %s, ready redirect to the host: %s", nextPath, host)
				if err != nil {
					w.WriteHeader(http.StatusBadGateway)
					errMsg := fmt.Sprintf("balancer error: %s", err.Error())
					w.Write([]byte(errMsg))
					return
				}
				httpProxy.lb.Inc(host)
				defer httpProxy.lb.Done(host)

				// 将前缀覆盖重写
				path = path[i:]
				logger.Debugf("rewrite the path: %s", path)
				req.URL.Path = path
				httpProxy.hostMap[host].ServeHTTP(w, req)
				break
			}
		}
	}
	if i < 0 {
		// 没有找到匹配的路由，返回404
		http.NotFound(w, req)
		return
	}

	err := r.configRate(httpProxy, nextPath)
	if err == ratelimit.LimiterAlreadyExists {
		return
	}
	if err != nil {
		logger.Error(err)
		return
	} else {
		logger.Debugf("create a new rateLimiter for path %s", nextPath)
	}
}

// 将此次接口访问记录到反向代理器中，用于配置限流
func (r *DefaultRouter) configRate(httpProxy *HTTPProxy, path string) error {
	if httpProxy == nil {
		errMsg := "default_router can't find the httpProxy"
		return errors.New(errMsg)
	}
	methods := httpProxy.methods
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
	} else {
		return ratelimit.LimiterAlreadyExists
	}
	return nil
}

func (r *DefaultRouter) invaildToken(proxy *HTTPProxy, api string) error {
	methods := proxy.methods
	limiter := methods[api]
	// 还未配置限流器，经过第一次访问之后，默认创建一个 qps 限流器
	if limiter == nil {
		return nil
	}
	timeout := limiter.GetTimeout()

	var err error
	if timeout == -1 {
		err = limiter.Take()
	} else {
		err = limiter.TakeWithTimeout(timeout)
	}
	return err
}