package consistence

import (
	reverseproxy "com.cheryl/cheryl/reverse_proxy"
)

type State struct {
	ProxyMap  *reverseproxy.ProxyMap
	RaftNode  *raftNodeInfo
	Hs        *HttpServer
}

type StateContext struct {
	State *State
}
