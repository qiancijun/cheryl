package reverseproxy

import (
	"log"
	"time"

	"com.cheryl/cheryl/utils"
)

var HealthCheckTimeout = 5 * time.Second

func (h *HTTPProxy) ReadAlive(url string) bool {
	h.RLock()
	defer h.RUnlock()
	return h.alive[url]
}

func (h *HTTPProxy) SetAlive(url string, alive bool) {
	h.Lock()
	defer h.Unlock()
	h.alive[url] = alive
}

func (h *HTTPProxy) HealthCheck() {
	for host := range h.hostMap {
		go h.healthCheck(host)
	}
}

func (h *HTTPProxy) healthCheck(host string) {
	ticker := time.Tick(HealthCheckTimeout)
	// for {
	// 	select {
	// 	case <- ticker:
	// 		if !utils.IsBackendAlive(host) && h.ReadAlive(host) {
	// 			log.Panicf("Site unreachable, remove %s from load balancer.", host)
	// 			h.SetAlive(host, false)
	// 			h.lb.Remove(host)
	// 		} else if utils.IsBackendAlive(host) && !h.ReadAlive(host) {
	// 			log.Printf("Site reachable, add %s to load balancer.", host)
	// 			h.SetAlive(host, true)
	// 			h.lb.Add(host)
	// 		}
	// 	}
	// }
	for range ticker {
		if !utils.IsBackendAlive(host) && h.ReadAlive(host) {
			log.Panicf("Site unreachable, remove %s from load balancer.", host)
			h.SetAlive(host, false)
			h.lb.Remove(host)
		} else if utils.IsBackendAlive(host) && !h.ReadAlive(host) {
			log.Printf("Site reachable, add %s to load balancer.", host)
			h.SetAlive(host, true)
			h.lb.Add(host)
		}
	}
}