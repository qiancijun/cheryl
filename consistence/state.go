package consistence

import reverseproxy "com.cheryl/cheryl/reverse_proxy"

type State struct {
	ProxyMap reverseproxy.ProxyMap
	Raft     *raftNodeInfo
	Hs       *HttpServer
}

type StateContext struct {
	Ctx *State
}