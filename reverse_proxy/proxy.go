package reverseproxy

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync"
	"time"

	"github.com/qiancijun/cheryl/acl"
	"github.com/qiancijun/cheryl/balancer"
	"github.com/qiancijun/cheryl/config"
	"github.com/qiancijun/cheryl/logger"
	ratelimit "github.com/qiancijun/cheryl/rate_limit"
	"github.com/qiancijun/cheryl/utils"
	jsoniter "github.com/json-iterator/go"
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
	HostMap  map[string]*httputil.ReverseProxy
	Pattern  string
	Lb       balancer.Balancer
	Alive    map[string]bool
	Methods  map[string]ratelimit.RateLimiter
	ProxyMap *ProxyMap
	sync.RWMutex
}

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

// 对每一个 URL 创建反向代理并且记录到 URL 树中
func NewHTTPProxy(pattern string, targetHosts []string, algo balancer.Algorithm) (*HTTPProxy, error) {
	hostMap := make(map[string]*httputil.ReverseProxy)
	alive := make(map[string]bool)
	methods := make(map[string]ratelimit.RateLimiter)

	hosts := make([]string, 0)
	for _, targetHost := range targetHosts {
		url, err := url.Parse(targetHost)
		if err != nil {
			return nil, err
		}
		log.Printf("%s has been created reverse proxy", url)
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
		logger.Debugf("success create reverproxy %s", host)
	}

	// 为代理配置一个负载均衡器
	lb, err := balancer.Build(algo, hosts)
	if err != nil {
		return nil, err
	}

	return &HTTPProxy{
		HostMap: hostMap,
		Lb:      lb,
		Alive:   alive,
		Pattern: pattern,
		Methods: methods,
	}, nil
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
	logger.Debug("%s will access the system", ip)
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
	// proxyMap.Router.Add(pattern, proxy)
	RouterSingleton.Add(pattern, proxy)
	proxy.HealthCheck()
}
