package reverseproxy

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync"
	"time"

	"github.com/qiancijun/cheryl/acl"
	"github.com/qiancijun/cheryl/balancer"
	"github.com/qiancijun/cheryl/logger"
	ratelimit "github.com/qiancijun/cheryl/rate_limit"
	"github.com/qiancijun/cheryl/utils"
)

const (
	XRealIP      string = "X-Real-IP"
	XProxy       string = "X-Proxy"
	ReverseProxy string = "Balancer-Reverse-Proxy"
)

/**
*	hostMap: 主机对反向代理的映射，其中的键值表示我们需要反向代理的主机
*	lb: 负载均衡器
* 	alive: 反向代理的主机是否处于健康状态
 */
type HTTPProxy struct {
	HostMap       map[string]*httputil.ReverseProxy
	Pattern       string
	Lb            balancer.Balancer
	Alive         map[string]bool
	Methods       map[string]ratelimit.RateLimiter
	HostsShutDown map[string]chan bool
	ShutDown      chan bool
	ProxyMap      *ProxyMap
	sync.RWMutex
}



// 对每一个 URL 创建反向代理并且记录到 URL 树中
func NewHTTPProxy(pattern string, targetHosts []string, algo balancer.Algorithm) (*HTTPProxy, error) {
	hostMap := make(map[string]*httputil.ReverseProxy)
	alive := make(map[string]bool)
	methods := make(map[string]ratelimit.RateLimiter)
	hostsShutDown := make(map[string]chan bool)

	hosts := make([]string, 0)
	for _, targetHost := range targetHosts {
		url, err := url.Parse(targetHost)
		if err != nil {
			return nil, err
		}
		logger.Debugf("%s has been created reverse proxy", url)
		proxy := httputil.NewSingleHostReverseProxy(url)

		originDirector := proxy.Director
		proxy.Director = func(r *http.Request) {
			originDirector(r)
			r.Header.Set(XProxy, ReverseProxy)
			r.Header.Set(XRealIP, utils.GetIP(r.RemoteAddr))
		}

		host := utils.GetHost(url)
		alive[host] = true
		hostMap[host] = proxy
		hosts = append(hosts, host)
		hostsShutDown[host] = make(chan bool, 0)
		logger.Debugf("success create reverproxy %s", host)
	}

	// 为代理配置一个负载均衡器
	lb, err := balancer.Build(algo, hosts)
	if err != nil {
		return nil, err
	}

	httpProxy := &HTTPProxy{
		HostMap: hostMap,
		Lb:      lb,
		Alive:   alive,
		Pattern: pattern,
		Methods: methods,
		ShutDown: make(chan bool),
		HostsShutDown: hostsShutDown,
	}

	// 监听是否收到Shutdown
	go httpProxy.shutDownCheck()

	return httpProxy, nil
}


func (h *HTTPProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	if !h.accessControl(utils.RemoteIp(r)) {
		w.WriteHeader(403)
		return
	}
	host, err := h.Lb.Balance(utils.GetIP(r.RemoteAddr))
	if err != nil {
		w.WriteHeader(http.StatusBadGateway)
		errMsg := fmt.Sprintf("balancer error: %s", err.Error())
		w.Write([]byte(errMsg))
		return
	}

	h.Lb.Inc(host)
	defer h.Lb.Done(host)
	h.HostMap[host].ServeHTTP(w, r)
}

func (h *HTTPProxy) accessControl(ip string) bool {
	logger.Debugf("%s will access the system", ip)
	radixTree := acl.AccessControlList
	ret := radixTree.Search(ip)
	return ret == ""
}

func (httpProxy *HTTPProxy) SetRateLimiter(info LimiterInfo) error {
	logger.Debugf("{SetRateLimiter} pathName: %s limiterType: %s volumn: %d speed: %d maxThread: %d", info.PathName, info.LimiterType, info.Volumn, info.Speed, info.MaxThread)
	methods := httpProxy.Methods
	httpProxy.Lock()
	defer httpProxy.Unlock()

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

func (httpProxy *HTTPProxy) invaildToken(api string) error {
	methods := httpProxy.Methods
	limiter := methods[api]
	// 还未配置限流器，经过第一次访问之后，默认创建一个 qps 限流器
	if limiter == nil {
		httpProxy.Lock()
		err := httpProxy.configRate(api)
		httpProxy.Unlock()
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

// 将此次接口访问记录到反向代理器中，用于配置限流
func (httpProxy *HTTPProxy) configRate(path string) error {
	if httpProxy == nil {
		errMsg := "default_router can't find the httpProxy"
		return errors.New(errMsg)
	}
	methods := httpProxy.Methods
	_, has := methods[path]
	if !has {

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

func (httpProxy *HTTPProxy) shutDownCheck() {
	select {
	case <- httpProxy.ShutDown: 
		// 关闭所有与 host 的心跳链接
		for k, c := range httpProxy.HostsShutDown {
			logger.Debugf("ready to close heartbeat for %s", k)
			c <- true
			logger.Debugf("close heatbeat for %s success", k)
		}
	}
}

func (httpProxy *HTTPProxy) ChangeLb(mode string) error {
	hosts := make([]string, 0)
	for k := range httpProxy.HostMap {
		hosts = append(hosts, k)
	}
	lb, err := balancer.Build(balancer.Algorithm(mode), hosts)
	if err != nil {
		logger.Warnf("can't create load balancer")
		return err
	}
	httpProxy.Lb = lb
	return nil
}