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
	Route(w http.ResponseWriter, r *http.Request) (*HTTPProxy, string)
}

var (
	RouterSingleton Router
	once = &sync.Once{}
)



func GetRouterInstance(name string) Router {
	if RouterSingleton == nil {
		switch name {
		case "default":
			once.Do(func ()  {
				RouterSingleton = &DefaultRouter{
					hosts: make(map[string]*HTTPProxy),
				}
			})
		}
	}
	return RouterSingleton
}