package consistence

import (
	"fmt"
	"net"
	"net/http"

	"com.cheryl/cheryl/config"
	"com.cheryl/cheryl/logger"
	reverseproxy "com.cheryl/cheryl/reverse_proxy"
)

func Start(config *config.Config) {
	proxyMap := reverseproxy.ProxyMap{
		Relations: make(map[string]*reverseproxy.HTTPProxy),
		Router: reverseproxy.GetRouterInstance("default"),
	}

	state := &State{
		ProxyMap: proxyMap,
	}

	stateContext := &StateContext{
		Ctx: state,
	}

	// 在本地端口开启 http 监听
	var httpListen net.Listener = createListener(config.HttpPort)
	
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
	state.Raft = raft
	// TODO: 如果是从节点，尝试加入到主节点中

	// TODO: 监听 leader
	for {
		select {
		case leader := <- state.Raft.leaderNotifych:
			if leader {
				logger.Debug("becomne leader, enable write api")
			} else {
				logger.Debug("becomne leader, disable write api")
			}
		}
	}
	
}

func createListener(port int) net.Listener {
	httpListen, err := net.Listen("tcp", fmt.Sprintf(":%d",port))
	if err != nil {
		logger.Errorf("listen %d failed: %s", port, err.Error())
	}
	logger.Infof("http server listen: %s", httpListen.Addr())
	return httpListen
}