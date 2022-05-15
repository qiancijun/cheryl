package consistence

import (
	"fmt"
	"net/http"
	"sync/atomic"
	"time"

	"com.cheryl/cheryl/config"
	"com.cheryl/cheryl/logger"
	reverseproxy "com.cheryl/cheryl/reverse_proxy"
	"com.cheryl/cheryl/utils"
	"github.com/hashicorp/raft"
	jsoniter "github.com/json-iterator/go"
)

const (
	ENABLE_WRITE_TRUE  = int32(1)
	ENABLE_WRITE_FALSE = int32(0)
)

var HealthCheckTimeout = 5 * time.Second

type HttpServer struct {
	Mux         *http.ServeMux
	Ctx         *StateContext
	address     []string
	enableWrite int32
}

func NewHttpServer(ctx *StateContext) *HttpServer {
	mux := http.NewServeMux()
	s := &HttpServer{
		Ctx:         ctx,
		Mux:         mux,
		enableWrite: ENABLE_WRITE_FALSE,
		address:     make([]string, 0),
	}
	s.address = append(s.address, utils.GetLocalIpAddress())
	mux.HandleFunc("/ping", s.doPing)
	mux.HandleFunc("/join", s.doJoin)
	mux.HandleFunc("/methods", s.doGetMethods)
	mux.HandleFunc("/limiter", s.doSetRateLimiter)
	mux.HandleFunc("/peers", s.doGetRaftClusterInfo)
	return s
}

func (h *HttpServer) doJoin(w http.ResponseWriter, r *http.Request) {
	variables := r.URL.Query()
	peerAddress := variables.Get("peerAddress")
	if peerAddress == "" {
		errMsg := "doJoin: invaild peerAddress"
		logger.Info(errMsg)
		fmt.Fprint(w, errMsg)
		return
	}
	logger.Debugf("peerAddress %s will join the cluster", peerAddress)
	addPeerFuture := h.Ctx.State.RaftNode.Raft.AddVoter(raft.ServerID(peerAddress), raft.ServerAddress(peerAddress), 0, 0)
	if err := addPeerFuture.Error(); err != nil {
		errMsg := fmt.Sprintf("Error joining peer to raft, peeraddress:%s, err:%v, code:%d", peerAddress, err, http.StatusInternalServerError)
		logger.Warn(errMsg)
		fmt.Fprint(w, errMsg)
		return
	}
	h.address = append(h.address, peerAddress)

	// 与远程主机保持心跳，如果失去心跳，从Raft集群中删除
	// go func() {
	// 	ticker := time.NewTicker(HealthCheckTimeout)
	// 	for range ticker.C {
	// 		if !utils.IsBackendAlive(peerAddress) {
	// 			removePeerFuture := h.Ctx.State.RaftNode.Raft.RemoveServer(raft.ServerID(peerAddress), 0, 0)
	// 			if err := removePeerFuture.Error(); err != nil {
	// 				errMsg := fmt.Sprintf("Error removing peer from raft, peerAddress:%s, err:%v", peerAddress, err)
	// 				logger.Warn(errMsg)
	// 				return
	// 			}
	// 			logger.Debugf("success remove raft Node, peerAddress: %s", peerAddress)
	// 			return
	// 		} else {
	// 			logger.Debugf("enable connect with peerAddress: %s", peerAddress)
	// 		}
	// 	}
	// }()
	fmt.Fprint(w, "ok")
}

// test methods
func (h *HttpServer) doPing(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, "pong")
}

// get the all reverse proxy infomation
func (h *HttpServer) doGetMethods(w http.ResponseWriter, r *http.Request) {
	relation := h.Ctx.State.ProxyMap.Relations

	type methodsInfo struct {
		Prefix      string   `json:"prefix"`
		MethodsPath []string `json:"methodsPath"`
	}

	ret := make([]methodsInfo, 0)
	for prefix, proxy := range relation {
		methods := proxy.Methods
		logger.Debugf("{doGetMethods} find prefix: %s", prefix)
		tmp := methodsInfo{
			Prefix:      prefix,
			MethodsPath: make([]string, 0),
		}
		for method := range methods {
			tmp.MethodsPath = append(tmp.MethodsPath, method)
			logger.Debugf("{doGetMethods} find method: %s%s", prefix, method)
		}
		ret = append(ret, tmp)
	}
	data, err := jsoniter.Marshal(ret)
	if err != nil {
		errMsg := fmt.Sprintf("{doGetMethods} can't marshal methods data: %s", err.Error())
		logger.Warn(errMsg)
		fmt.Fprint(w, errMsg)
		return
	}
	_, err = w.Write(data)
	if err != nil {
		errMsg := fmt.Sprintf("{doGetMethods} can't write json data: %s", err.Error())
		logger.Warn(errMsg)
		fmt.Fprint(w, errMsg)
	}
}

func (h *HttpServer) doSetRateLimiter(w http.ResponseWriter, r *http.Request) {
	if !h.checkWritePermission() {
		w.Write(Error(500, "write method not allowed").Marshal())
		return
	}
	type ReqData struct {
		Prefix string                   `json:"prefix"`
		Info   reverseproxy.LimiterInfo `json:"limiterInfo"`
	}
	var req ReqData
	if err := jsoniter.NewDecoder(r.Body).Decode(&req); err != nil {
		r.Body.Close()
		errMsg := fmt.Sprintf("can't receive the json data: %s", err.Error())
		logger.Warn(errMsg)
		ret := Error(500, errMsg)
		w.Write(ret.Marshal())
		return
	}
	// first: write in local, if success, send logEntry to the raft cluster
	httpProxy, has := h.Ctx.State.ProxyMap.Relations[req.Prefix]
	if !has {
		errMsg := fmt.Sprintf("can't find the httpProxy: %s", req.Prefix)
		logger.Warn(errMsg)
		w.Write(Error(500, errMsg).Marshal())
		return
	}

	err := h.Ctx.State.ProxyMap.Router.SetRateLimiter(httpProxy, req.Info)
	if err != nil {
		errMsg := fmt.Sprintf("can't set the %s%s RateLimiter: %s", req.Prefix, req.Info.PathName, err.Error())
		logger.Warn(errMsg)
		w.Write(Error(500, errMsg).Marshal())
		return
	}

	// second: send logEntry to the raft cluster
	err = h.Ctx.writeLogEntry(2, req.Prefix, map[string]string{}, config.Location{}, req.Info)
	if err != nil {
		errMsg := fmt.Sprintf("can't apply log entry: %s", err.Error())
		logger.Warn(errMsg)
		ret := Error(500, errMsg)
		w.Write(ret.Marshal())
		return
	}

	// third: write success info to the frontend
	w.Write(Ok().Marshal())
}

func (h *HttpServer) doGetRaftClusterInfo(w http.ResponseWriter, r *http.Request) {
	servers := h.Ctx.State.RaftNode.Raft.GetConfiguration().Configuration().Servers
	leader := h.Ctx.State.RaftNode.Raft.Leader()
	logger.Debugf("{doGetRaftClusterInfo} the leader address is: %s", leader)
	type Response struct {
		Id       string `json:"serverId"`
		Address  string `json:"serverAddress"`
		IsLeader bool   `json:"isLeader"`
	}
	res := make([]Response, 0)
	for _, server := range servers {
		isLeader := (server.Address == leader)
		r := Response{
			Id:       string(server.ID),
			Address:  string(server.Address),
			IsLeader: isLeader,
		}
		res = append(res, r)
	}
	// data, err := jsoniter.Marshal(res)
	// if err != nil {
	// 	errMsg := fmt.Sprintf("can't marshal raft cluster peers: %s", err.Error())
	// 	logger.Warn(errMsg)
	// 	w.Write(Error(500, errMsg).Marshal())
	// 	return
	// }
	w.Write(Ok().Put("data", res).Marshal())
}

func (h *HttpServer) checkWritePermission() bool {
	return atomic.LoadInt32(&h.enableWrite) == ENABLE_WRITE_TRUE
}

func (h *HttpServer) SetWriteFlag(flag bool) {
	if flag {
		atomic.StoreInt32(&h.enableWrite, ENABLE_WRITE_TRUE)
	} else {
		atomic.StoreInt32(&h.enableWrite, ENABLE_WRITE_FALSE)
	}
}
