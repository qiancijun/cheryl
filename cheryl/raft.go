package cheryl

import (
	"fmt"
	"net"
	"net/http"
	"path/filepath"
	"time"

	jsoniter "github.com/json-iterator/go"
	"github.com/qiancijun/cheryl/config"
	"github.com/qiancijun/cheryl/logger"
	reverseproxy "github.com/qiancijun/cheryl/reverse_proxy"
	"github.com/qiancijun/cheryl/utils"
)

var (
	LeaderCheckTimeout = 10 * time.Second
	Context *State
)

func Start(conf *config.CherylConfig) {

	proxyMap := reverseproxy.NewProxyMap()
	logger.Debug("init proxyMap success")

	state := &State{
		ProxyMap: proxyMap,
	}
	Context = state

	stateContext := &StateContext{
		State: state,
	}

	init, _ := utils.PathExist(filepath.Join(conf.Raft.DataDir, conf.Name))

	// 在本地端口开启 http 监听
	httpListen, err := createListener(conf.HttpPort)
	if err != nil {
		logger.Errorf("listen %d failed", conf.HttpPort)
	}

	// 创建 http Server
	httpServer := newHttpServer(stateContext)
	state.Hs = httpServer
	go func() {
		http.Serve(httpListen, httpServer.Mux)
	}()

	// 创建 raft 节点
	raft, err := createRaftNode(conf, stateContext)
	if err != nil {
		logger.Errorf("create new raft node failed: %s", err.Error())
	}
	state.RaftNode = raft
	// 如果是从节点，尝试加入到主节点中
	if !conf.Raft.IsLeader && conf.Raft.LeaderAddress != "" {
		err := joinRaftCluster(conf)
		if err != nil {
			logger.Errorf("join raft cluster failed: %s", err.Error())
		}
		logger.Infof("join raft cluster success, %s", state.RaftNode.Raft.String())
	}

	// 监听 leader
	go func() {
		for leader := range state.RaftNode.leaderNotifych {
			if leader || conf.Raft.IsLeader {
				if !init {
					logger.Debugf("the node %s is first time start, ready create Proxy from config", conf.Name)
					createProxy(stateContext, conf)
				}
				httpServer.SetWriteFlag(true)
				logger.Debug("become leader, enable write api")
			} else {
				logger.Debug("become follower, disable write api")
				httpServer.SetWriteFlag(false)
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

func createProxy(ctx *StateContext, conf *config.CherylConfig) {
	logger.Debugf("{createProxy} %s will createProxy", conf.Name)
	for _, l := range conf.Location {
		createProxyWithLocation(ctx, l)
	}
}

func createProxyWithLocation(ctx *StateContext, l config.Location) {
	err := ctx.State.ProxyMap.AddProxyWithLocation(l)
	if err != nil {
		logger.Errorf("create proxy error: %s", err)
	}
	data, err := jsoniter.Marshal(l)
	if err != nil {
		logger.Warnf("can't marshal location: %s", err.Error())
		return
	}
	err = ctx.writeLogEntry(1, data)
	if err != nil {
		logger.Warnf("{createProxyWithLocation} write logEntry failed: %s", err.Error())
	}
}

func startRouter(ctx *StateContext, conf *config.CherylConfig) {
	cfg := config.GetConfig()
	r := http.NewServeMux()
	// router := ctx.State.ProxyMap.Router
	router := reverseproxy.GetRouterInstance(conf.RouterType)
	r.Handle("/", router)
	svr := http.Server{
		// Addr:    fmt.Sprintf(":%d", conf.Port),
		Handler:           r,
		ReadHeaderTimeout: time.Duration(cfg.ReadHeaderTimeout) * time.Second,
		ReadTimeout:       time.Duration(cfg.ReadTimeout) * time.Second,
		IdleTimeout:       time.Duration(cfg.IdleTimeout) * time.Second,
	}

	l, err := net.Listen("tcp", fmt.Sprintf(":%d", conf.Port))
	if err != nil {
		logger.Errorf("can't create listen on %d", conf.Port)
	}

	if conf.Schema == "http" {
		if err := svr.Serve(l); err != nil {
			logger.Errorf("listen and serve error: %s", err)
		}
	} else if conf.Schema == "https" {
		if err := svr.ServeTLS(l, conf.SSLCertificate, conf.SSLCertificateKey); err != nil {
			logger.Errorf("listen and serve error: %s", err)
		}
	}
}

func (ctx *StateContext) writeLogEntry(optType uint16, data []byte) error {
	event := LogEntry{optType, data}
	eventBytes, err := event.Encode()
	if err != nil {
		logger.Warnf("{writeLogEntry} json marshal failed: %s", err.Error())
		return err
	}
	logger.Debug("{writeLogEntry} marshal log success")
	applyFuture := ctx.State.RaftNode.Raft.Apply(eventBytes, 5*time.Second)
	if err := applyFuture.Error(); err != nil {
		logger.Warnf("raft apply failed: %s", err.Error())
		return err
	}
	idx := applyFuture.Index()
	logger.Debugf("the new raft index: %d", idx)
	return nil
}
