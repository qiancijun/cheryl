package consistence

import reverseproxy "com.cheryl/cheryl/reverse_proxy"

type State struct {
	ProxyMap reverseproxy.ProxyMap
	Raft     *raftNodeInfo
}

type StateContext struct {
	Ctx *State
}
