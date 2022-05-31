package reverseproxy

import (
	"time"

	"github.com/qiancijun/cheryl/logger"
	"github.com/qiancijun/cheryl/utils"
)

var HealthCheckTimeout = 5 * time.Second

func (h *HTTPProxy) ReadAlive(url string) bool {
	h.RLock()
	defer h.RUnlock()
	return h.Alive[url]
}

func (h *HTTPProxy) SetAlive(url string, alive bool) {
	h.Lock()
	defer h.Unlock()
	h.Alive[url] = alive
}

func (h *HTTPProxy) HealthCheck() {
	for host := range h.HostMap {
		go h.healthCheck(host)
	}
}

func (h *HTTPProxy) healthCheck(host string) {
	ticker := time.Tick(HealthCheckTimeout)
	for range ticker {
		if !utils.IsBackendAlive(host) && h.ReadAlive(host) {
			logger.Warnf("Site unreachable, remove %s from load balancer.", host)
			h.SetAlive(host, false)
			h.Lb.Remove(host)
		} else if utils.IsBackendAlive(host) && !h.ReadAlive(host) {
			logger.Warnf("Site reachable, add %s to load balancer.", host)
			h.SetAlive(host, true)
			h.Lb.Add(host)
		}
	}
}
