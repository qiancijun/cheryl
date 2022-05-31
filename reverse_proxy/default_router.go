package reverseproxy

import (
	"fmt"
	"net/http"
	"sync"

	"github.com/qiancijun/cheryl/acl"
	"github.com/qiancijun/cheryl/filter"
	"github.com/qiancijun/cheryl/logger"
	"github.com/qiancijun/cheryl/utils"
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

/*
	执行方法的顺序：
	1. 判断 ip 是否在黑名单内 （acl）
	2. 执行一遍 FilterChain 的方法 
	3. 根据 path 找到反向代理
	4. 限流
	5. 根据反向代理中的主机路径，进行负载均衡
	6. 找出一个转发的主机，转发请求
*/
func (r *DefaultRouter) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	logger.Infof("%s can't catch any path", req.URL)
	// accessControlList
	isDeny := acl.AccessControlList.AccessControl(utils.RemoteIp(req))
	if isDeny {
		w.WriteHeader(403)
		return
	}

	// filterChain
	err := filter.ExecuteFilterChain(w, req)
	if err != nil {
		w.Write([]byte(err.Error()))
		return
	}

	// route
	httpProxy, Realpath := r.Route(w, req)
	if httpProxy == nil {
		w.WriteHeader(404)
		return
	}

	// Rate Limit
	err = httpProxy.invaildToken(Realpath)
	if err != nil {
		errMsg := fmt.Sprintf("route error: %s", err.Error())
		logger.Debug(errMsg)
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(errMsg))
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



