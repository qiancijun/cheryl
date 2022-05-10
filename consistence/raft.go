package consistence

import (
	"fmt"
	"net"
	"net/http"
	"time"

	"com.cheryl/cheryl/balancer"
	"com.cheryl/cheryl/config"
	"com.cheryl/cheryl/logger"
	reverseproxy "com.cheryl/cheryl/reverse_proxy"
	jsoniter "github.com/json-iterator/go"
)

func Start(config *config.Config) {
	proxyMap := reverseproxy.ProxyMap{
		Relations: make(map[string]*reverseproxy.HTTPProxy),
		Router:    reverseproxy.GetRouterInstance("default"),
	}

	state := &State{
		ProxyMap: proxyMap,
	}

	stateContext := &StateContext{
		State: state,
	}

	// 在本地端口开启 http 监听
	httpListen, err := createListener(config.HttpPort)
	if err != nil {
		logger.Errorf("listen %s failed", config.HttpPort)
	}

	// 创建 http Server
	httpServer := NewHttpServer(stateContext)
	state.Hs = httpServer
	go func() {
		http.Serve(httpListen, httpServer.Mux)
	}()

	// 创建 raft 节点
	raft, err := Make(config, stateContext)
	if err != nil {
		logger.Errorf("create new raft node failed: %s", err.Error())
	}
	state.RaftNode = raft
	// TODO: 如果是从节点，尝试加入到主节点中
	if !config.Raft.IsLeader && config.Raft.LeaderAddress != "" {
		err := JoinRaftCluster(config)
		if err != nil {
			logger.Errorf("join raft cluster failed: %s", err.Error())
		}
		logger.Infof("join raft cluster success, %s", state.RaftNode.Raft.String())
	}

	// TODO: 监听 leader
	go func() {
		for {
			select {
			case leader := <-state.RaftNode.leaderNotifych:
				if leader {
					logger.Debug("becomne leader, enable write api")
				} else {
					logger.Debug("becomne leader, disable write api")
				}
			}
		}
	}()

	startProxy(stateContext, config)

}

func createListener(port int) (net.Listener, error) {
	httpListen, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		logger.Warnf("listen %d failed: %s", port, err.Error())
		return nil, err
	}
	logger.Infof("http server listen: %s", httpListen.Addr())
	return httpListen, nil
}

func startProxy(ctx *StateContext, conf *config.Config) {
	r := http.NewServeMux()
	// TODO: 根据配置更换路由器类型
	router := reverseproxy.GetRouterInstance("default")
	for _, l := range conf.Location {
		httpProxy, err := reverseproxy.NewHTTPProxy(l.Pattern, l.ProxyPass, balancer.Algorithm(l.BalanceMode))
		if err != nil {
			logger.Errorf("create proxy error: %s", err)
		}
		router.Add(l.Pattern, httpProxy)
		if conf.HealthCheck {
			httpProxy.HealthCheck()
		}
		// 写入 LogEntry
		writeLogEntry(ctx, 1, l.Pattern, l)
		// r.Handle(l.Pattern, httpProxy)
	}
	r.Handle("/", router)

	svr := http.Server {
		Addr: fmt.Sprintf(":%d", conf.Port),
		Handler: r,
	}
	if conf.Schema == "http" {
		if err := svr.ListenAndServe(); err != nil {
			logger.Errorf("listen and serve error: %s", err)
		}
	} else if conf.Schema == "https" {
		if err := svr.ListenAndServeTLS(conf.SSLCertificate, conf.SSLCertificateKey); err != nil {
			logger.Errorf("listen and serve error: %s", err)
		}
	}
}

func writeLogEntry(ctx *StateContext, opt int, key string, value interface{}) error {
	event := LogEntryData{opt, key, value}
	eventBytes, err := jsoniter.Marshal(event)
	if err != nil {
		logger.Warnf("json marshal failed: %s", err.Error())
		return err
	}
	applyFuture := ctx.State.RaftNode.Raft.Apply(eventBytes, 5 * time.Second)
	if err := applyFuture.Error(); err != nil {
		logger.Warnf("raft apply failed: %s", err.Error())
		return err
	}
	return nil
}