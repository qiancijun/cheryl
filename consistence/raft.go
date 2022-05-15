package consistence

import (
	"fmt"
	"net"
	"net/http"
	"path/filepath"
	"time"

	"com.cheryl/cheryl/balancer"
	"com.cheryl/cheryl/config"
	"com.cheryl/cheryl/logger"
	reverseproxy "com.cheryl/cheryl/reverse_proxy"
	"com.cheryl/cheryl/utils"
	jsoniter "github.com/json-iterator/go"
)

var (
	LeaderCheckTimeout = 10 * time.Second
)

func Start(conf *config.Config) {
	// TODO: 类型封装
	proxyMap := reverseproxy.ProxyMap{
		Relations: make(map[string]*reverseproxy.HTTPProxy),
		Router:    reverseproxy.GetRouterInstance("default"),
		Locations: make(map[string]*config.Location),
		Infos:     reverseproxy.Info{
			RouterType: "default",
		},
		Limiters: make(map[string][]reverseproxy.LimiterInfo),
	}

	state := &State{
		ProxyMap: &proxyMap,
	}

	stateContext := &StateContext{
		State: state,
	}

	init, _ := utils.PathExist(filepath.Join(conf.Raft.DataDir, conf.Name))

	// 在本地端口开启 http 监听
	httpListen, err := createListener(conf.HttpPort)
	if err != nil {
		logger.Errorf("listen %s failed", conf.HttpPort)
	}

	// 创建 http Server
	httpServer := NewHttpServer(stateContext)
	state.Hs = httpServer
	go func() {
		http.Serve(httpListen, httpServer.Mux)
	}()

	// 创建 raft 节点
	raft, err := Make(conf, stateContext)
	if err != nil {
		logger.Errorf("create new raft node failed: %s", err.Error())
	}
	state.RaftNode = raft
	// 如果是从节点，尝试加入到主节点中
	if !conf.Raft.IsLeader && conf.Raft.LeaderAddress != "" {
		err := JoinRaftCluster(conf)
		if err != nil {
			logger.Errorf("join raft cluster failed: %s", err.Error())
		}
		logger.Infof("join raft cluster success, %s", state.RaftNode.Raft.String())
	}

	// 监听 leader
	go func() {
		for leader := range state.RaftNode.leaderNotifych {
			if leader && conf.Raft.IsLeader {
				if !init {
					logger.Debugf("the node %s is first time start, ready create Proxy from config", conf.Name)
					createProxy(stateContext, conf)
				}
				httpServer.SetWriteFlag(true)
				logger.Debug("become leader, enable write api")
			} else {
				logger.Debug("become follower, disable write api")
				httpServer.SetWriteFlag(true)
			}
		}
	}()
	startRouter(stateContext, conf)
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

func createProxy(ctx *StateContext, conf *config.Config) {
	logger.Debugf("{createProxy} %s will createProxy", conf.Name)
	for _, l := range conf.Location {
		createProxyWithLocation(ctx, l)
	}
}

func createProxyWithLocation(ctx *StateContext, l *config.Location) {
	httpProxy, err := reverseproxy.NewHTTPProxy(l.Pattern, l.ProxyPass, balancer.Algorithm(l.BalanceMode))
	if err != nil {
		logger.Errorf("create proxy error: %s", err)
	}
	httpProxy.ProxyMap = ctx.State.ProxyMap
	ctx.State.ProxyMap.AddRelations(l.Pattern, httpProxy, l)
	httpProxy.HealthCheck()
	err = ctx.writeLogEntry(1, l.Pattern, make(map[string]string, 0), *l, reverseproxy.LimiterInfo{})
	if err != nil {
		logger.Warnf("{createProxyWithLocation} write logEntry failed: %s", err.Error())
	}
}

func startRouter(ctx *StateContext, conf *config.Config) {
	r := http.NewServeMux()
	router := ctx.State.ProxyMap.Router
	r.Handle("/", router)
	svr := http.Server{
		Addr:    fmt.Sprintf(":%d", conf.Port),
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

func (ctx *StateContext) writeLogEntry(opt int, key string, value map[string]string, location config.Location, limiterInfo reverseproxy.LimiterInfo) error {
	event := LogEntryData{opt, key, value, location, limiterInfo}
	logger.Debugf("{writeLogEntry} the new event: %s %v", key, value)
	eventBytes, err := jsoniter.Marshal(event)
	if err != nil {
		logger.Warnf("{writeLogEntry} json marshal failed: %s", err.Error())
		return err
	}
	logger.Debugf("{writeLogEntry} marshal log success %s", string(eventBytes))
	applyFuture := ctx.State.RaftNode.Raft.Apply(eventBytes, 5*time.Second)
	if err := applyFuture.Error(); err != nil {
		logger.Warnf("raft apply failed: %s", err.Error())
		return err
	}
	idx := applyFuture.Index()
	logger.Debugf("the new raft index: %d", idx)
	return nil
}
