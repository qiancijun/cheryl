package cheryl

import (
	reverseproxy "github.com/qiancijun/cheryl/reverse_proxy"
)

type State struct {
	ProxyMap  *reverseproxy.ProxyMap
	RaftNode  *raftNodeInfo
	Hs        *HttpServer
}

type StateContext struct {
	State *State
}
