package reverseproxy

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync"

	"com.cheryl/cheryl/acl"
	"com.cheryl/cheryl/balancer"
	"com.cheryl/cheryl/config"
	"com.cheryl/cheryl/logger"
	rateLimit "com.cheryl/cheryl/rate_limit"
	"com.cheryl/cheryl/utils"
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
	Methods  map[string]rateLimit.RateLimiter
	ProxyMap *ProxyMap
	sync.RWMutex
}

type ProxyMap struct {
	sync.RWMutex
	Relations map[string]*HTTPProxy `json:"-"`
	Locations map[string]config.Location
	Router    Router `json:"-"`
	Limiters  map[string][]LimiterInfo
	Infos     Info
	Acl       *acl.RadixTree
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
	rt := acl.NewRadixTree()
	router := GetRouterInstance("default").(*DefaultRouter)
	router.acl = rt
	return &ProxyMap{
		Relations: make(map[string]*HTTPProxy),
		Router:    router,
		Locations: make(map[string]config.Location),
		Infos: Info{
			RouterType: "default",
		},
		Limiters: make(map[string][]LimiterInfo),
		Acl:      rt,
	}
}

// 对每一个 URL 创建反向代理并且记录到 URL 树中
func NewHTTPProxy(pattern string, targetHosts []string, algo balancer.Algorithm) (*HTTPProxy, error) {
	hostMap := make(map[string]*httputil.ReverseProxy)
	alive := make(map[string]bool)
	methods := make(map[string]rateLimit.RateLimiter)

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
	radixTree := h.ProxyMap.Acl
	ret := radixTree.Search(ip)
	return ret == ""
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
	proxyMap.Router.Add(pattern, proxy)
	proxy.HealthCheck()
}
