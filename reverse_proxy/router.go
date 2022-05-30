package reverseproxy

import (
	"net/http"
	"sync"
)

// 路由转发器
type Router interface {
	Add(string, *HTTPProxy)
	Remove(string)
	HasPrefix(string) bool
	ServeHTTP(w http.ResponseWriter, r *http.Request)
	SetRateLimiter(*HTTPProxy, LimiterInfo) error
	Route(w http.ResponseWriter, r *http.Request) (*HTTPProxy, string)
	ConfigRate(*HTTPProxy, string) error
}

var (
	routerSingleton Router
	once = &sync.Once{}
)

func GetRouterInstance(name string) Router {
	if routerSingleton == nil {
		switch name {
		case "default":
			once.Do(func ()  {
				routerSingleton = &DefaultRouter{
					hosts: make(map[string]*HTTPProxy),
				}
			})
		}
	}
	return routerSingleton
}