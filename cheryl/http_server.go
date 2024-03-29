package cheryl

import (
	"fmt"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/hashicorp/raft"
	jsoniter "github.com/json-iterator/go"
	"github.com/qiancijun/cheryl/acl"
	"github.com/qiancijun/cheryl/balancer"
	"github.com/qiancijun/cheryl/config"
	"github.com/qiancijun/cheryl/logger"
	ratelimit "github.com/qiancijun/cheryl/rate_limit"
	reverseproxy "github.com/qiancijun/cheryl/reverse_proxy"
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

func newHttpServer(ctx *StateContext) *HttpServer {
	mux := http.NewServeMux()
	s := &HttpServer{
		Ctx:         ctx,
		Mux:         mux,
		enableWrite: ENABLE_WRITE_FALSE,
		address:     make([]string, 0),
	}

	mux.HandleFunc("/ping", s.doPing)
	mux.HandleFunc("/join", s.doJoin)
	mux.HandleFunc("/methods", s.doGetMethods)
	mux.HandleFunc("/limiter", s.doSetRateLimiter)
	mux.HandleFunc("/peers", s.doGetRaftClusterInfo)
	mux.HandleFunc("/info", s.doGetInfo)
	mux.HandleFunc("/proxy", s.doGetProxy)
	mux.HandleFunc("/methodInfo", s.doGetMehtodLimiter)
	mux.HandleFunc("/addProxy", s.doAddProxy)
	mux.HandleFunc("/addHost", s.doAddHost)
	mux.HandleFunc("/acl", s.doHandleAcl)
	mux.HandleFunc("/getAcl", s.doGetAccessControlList)
	mux.HandleFunc("/getRateLimiterType", s.doGetRateLimiterType)
	mux.HandleFunc("/removeProxy", s.doRemoveProxy)
	mux.HandleFunc("/removeHost", s.doRemoveHost)
	mux.HandleFunc("/balancerMode", s.doGetBalancerMode)
	mux.HandleFunc("/changeLb", s.doChangeLb)
	mux.Handle("/", http.FileServer(http.Dir("static")))
	return s
}

func (h *HttpServer) doJoin(w http.ResponseWriter, r *http.Request) {
	variables := r.URL.Query()
	peerAddress := variables.Get("peerAddress")
	name := variables.Get("name")
	if peerAddress == "" {
		errMsg := "doJoin: invaild peerAddress"
		logger.Info(errMsg)
		fmt.Fprint(w, errMsg)
		return
	}
	logger.Debugf("peerAddress %s will join the cluster", peerAddress)
	addPeerFuture := h.Ctx.State.RaftNode.Raft.AddVoter(raft.ServerID(name), raft.ServerAddress(peerAddress), 0, 0)
	if err := addPeerFuture.Error(); err != nil {
		errMsg := fmt.Sprintf("Error joining peer to raft, peeraddress:%s, err:%v, code:%d", peerAddress, err, http.StatusInternalServerError)
		logger.Warn(errMsg)
		fmt.Fprint(w, errMsg)
		return
	}
	h.address = append(h.address, peerAddress)
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
	var info reverseproxy.LimiterInfo
	if err := jsoniter.NewDecoder(r.Body).Decode(&info); err != nil {
		errMsg := fmt.Sprintf("can't receive the json data: %s", err.Error())
		logger.Warn(errMsg)
		ret := Error(500, errMsg)
		w.Write(ret.Marshal())
		return
	}
	data, err := jsoniter.Marshal(info)
	if err != nil {
		logger.Warnf("can't marshal json data: %s", err.Error())
		w.WriteHeader(500)
		return
	}

	// first: write in local, if success, send logEntry to the raft cluster
	httpProxy, has := h.Ctx.State.ProxyMap.Relations[info.Prefix]
	if !has {
		errMsg := fmt.Sprintf("can't find the httpProxy: %s", info.Prefix)
		logger.Warn(errMsg)
		w.Write(Error(500, errMsg).Marshal())
		return
	}
	logger.Debugf("receive limiter info: %v", info)

	// err := h.Ctx.State.ProxyMap.Router.SetRateLimiter(httpProxy, req.Info)
	// second: send logEntry to the raft cluster

	if err = h.Ctx.writeLogEntry(2, data); err != nil {
		errMsg := fmt.Sprintf("can't apply log entry: %s", err.Error())
		logger.Warn(errMsg)
		ret := Error(500, errMsg)
		w.Write(ret.Marshal())
		return
	}

	if err = httpProxy.SetRateLimiter(info); err != nil {
		errMsg := fmt.Sprintf("can't set the %s%s RateLimiter: %s", info.Prefix, info.PathName, err.Error())
		logger.Warn(errMsg)
		w.Write(Error(500, errMsg).Marshal())
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
	w.Write(Ok().Put("data", res).Marshal())
}

func (h *HttpServer) doGetInfo(w http.ResponseWriter, r *http.Request) {
	type Response struct {
		Name        string `json:"name"`
		RaftAddress string `json:"raftAddress"`
		IsLeader    bool   `json:"isLeader"`
		ProxyPort   int    `json:"proxyPort"`
	}
	conf := config.GetConfig()
	name := conf.Name
	address := conf.Raft.RaftTCPAddress
	leader := h.Ctx.State.RaftNode.Raft.Leader()
	preoxyPort := conf.Port
	res := Response{
		Name:        name,
		RaftAddress: address,
		IsLeader:    address == string(leader),
		ProxyPort:   preoxyPort,
	}
	w.Write(Ok().Put("info", res).Marshal())
}

/*
{
	"/api": {
		"balance_mode": "round-robin",
		"hosts": [];
	}
}
*/
func (h *HttpServer) doGetProxy(w http.ResponseWriter, r *http.Request) {
	type host struct {
		Host  string `json:"host"`
		Alive bool   `json:"alive"`
	}
	type Response struct {
		BalancerMode string `json:"balancerMode"`
		Hosts        []host `json:"hosts"`
	}
	data := make(map[string]Response)
	for k, v := range h.Ctx.State.ProxyMap.Relations {
		proxy := Response{}
		proxy.BalancerMode = v.Lb.Mode()
		proxy.Hosts = make([]host, 0)
		for h := range v.HostMap {
			proxy.Hosts = append(proxy.Hosts, host{h, v.Alive[h]})
		}
		data[k] = proxy
	}
	w.Write(Ok().Put("data", data).Marshal())
}

func (h *HttpServer) doGetMehtodLimiter(w http.ResponseWriter, r *http.Request) {
	type ReqData struct {
		Pattern string `json:"pattern"`
		Method  string `json:"method"`
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
	limiter := h.Ctx.State.ProxyMap.Relations[req.Pattern].Methods[req.Method]
	if limiter == nil {
		w.Write(Error(404, "没有找到该方法").Marshal())
		return
	}
	type Response struct {
		Type    string        `json:"type"`
		Speed   int64         `json:"speed"`
		Volumn  int           `json:"volumn"`
		Timeout time.Duration `json:"timeout"`
	}
	var tp string
	switch limiter.(type) {
	case *ratelimit.QpsRateLimiter:
		tp = "qps"
	default:
		tp = "unknown"
	}
	res := Response{
		Type:    tp,
		Speed:   limiter.GetSpeed(),
		Volumn:  limiter.GetVolumn(),
		Timeout: limiter.GetTimeout(),
	}
	w.Write(Ok().Put("method", res).Marshal())
}

func (h *HttpServer) doAddProxy(w http.ResponseWriter, r *http.Request) {
	if !h.checkWritePermission() {
		w.Write(Error(500, "write method not allowed").Marshal())
		return
	}
	var location config.Location
	if err := jsoniter.NewDecoder(r.Body).Decode(&location); err != nil {
		r.Body.Close()
		errMsg := fmt.Sprintf("can't receive the json data: %s", err.Error())
		logger.Warn(errMsg)
		ret := Error(500, errMsg)
		w.Write(ret.Marshal())
		return
	}
	logger.Debug(location)
	err := validLocation(location)
	if err != nil {
		logger.Warn(err.Error())
		w.Write(Error(500, err.Error()).Marshal())
		return
	}
	createProxyWithLocation(h.Ctx, location)
	w.Write(Ok().Marshal())
}

func (h *HttpServer) doAddHost(w http.ResponseWriter, r *http.Request) {
	var req HostLog
	if err := jsoniter.NewDecoder(r.Body).Decode(&req); err != nil {
		r.Body.Close()
		errMsg := fmt.Sprintf("can't receive the json data: %s", err.Error())
		logger.Warn(errMsg)
		ret := Error(500, errMsg)
		w.Write(ret.Marshal())
		return
	}
	data, err := jsoniter.Marshal(req)
	if err != nil {
		errMsg := fmt.Sprintf("can't resolve json data: %s", err.Error())
		logger.Warn(errMsg)
		ret := Error(500, errMsg)
		w.Write(ret.Marshal())
		return
	}

	if err = h.Ctx.State.ProxyMap.AddProxy(req.Pattern, req.Host); err != nil {
		w.Write(Error(500, err.Error()).Marshal())
		return
	}
	if err = h.Ctx.writeLogEntry(6, data); err != nil {
		errMsg := fmt.Sprintf("can't apply log entry: %s", err.Error())
		logger.Warn(errMsg)
		ret := Error(500, errMsg)
		w.Write(ret.Marshal())
		return
	}
	w.Write(Ok().Marshal())
}

func (h *HttpServer) doHandleAcl(w http.ResponseWriter, r *http.Request) {
	if !h.checkWritePermission() {
		w.Write(Error(500, "write method not allowed").Marshal())
		return
	}

	var req AclLog
	if err := jsoniter.NewDecoder(r.Body).Decode(&req); err != nil {
		logger.Warnf("can't resolve json data: %s", err.Error())
		w.WriteHeader(400)
		return
	}

	data, err := jsoniter.Marshal(req)
	if err != nil {
		errMsg := fmt.Sprintf("can't resolve json data: %s", err.Error())
		logger.Warn(errMsg)
		ret := Error(500, errMsg)
		w.Write(ret.Marshal())
		return
	}
	logger.Debugf("{doHandleAcl} receive opt type: %d, ipAddress: %s", req.Pattern, req.IpAddress)
	if req.Pattern == 0 {
		// delete
		err = acl.AccessControlList.Delete(req.IpAddress)
		if err != nil {
			w.Write(Error(500, err.Error()).Marshal())
			return
		}
	} else {
		err = acl.AccessControlList.Add(req.IpAddress, req.IpAddress)
		if err != nil {
			w.Write(Error(500, err.Error()).Marshal())
			return
		}
	}
	if err := h.Ctx.writeLogEntry(3, data); err != nil {
		errMsg := fmt.Sprintf("can't apply log entry: %s", err.Error())
		logger.Warn(errMsg)
		ret := Error(500, errMsg)
		w.Write(ret.Marshal())
		return
	}
	w.Write(Ok().Marshal())
}

func (h *HttpServer) doGetAccessControlList(w http.ResponseWriter, r *http.Request) {
	list := acl.AccessControlList.GetBlackList()
	w.Write(Ok().Put("list", list).Marshal())
}

func (h *HttpServer) doGetRateLimiterType(w http.ResponseWriter, r *http.Request) {
	ret := ratelimit.GetLimiterType()
	w.Write(Ok().Put("list", ret).Marshal())
}

func (h *HttpServer) doRemoveProxy(w http.ResponseWriter, r *http.Request) {
	type ReqData struct {
		Pattern string `json:"pattern"`
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
	err := h.Ctx.State.ProxyMap.RemoveProxy(req.Pattern)
	if err != nil {
		w.Write(Error(500, err.Error()).Marshal())
		return
	}
	if err := h.Ctx.writeLogEntry(4, []byte(req.Pattern)); err != nil {
		errMsg := fmt.Sprintf("can't apply log entry: %s", err.Error())
		logger.Warn(errMsg)
		ret := Error(500, errMsg)
		w.Write(ret.Marshal())
		return
	}
	w.Write(Ok().Marshal())
}

func (h *HttpServer) doRemoveHost(w http.ResponseWriter, r *http.Request) {
	var req HostLog
	if err := jsoniter.NewDecoder(r.Body).Decode(&req); err != nil {
		r.Body.Close()
		errMsg := fmt.Sprintf("can't receive the json data: %s", err.Error())
		logger.Warn(errMsg)
		ret := Error(500, errMsg)
		w.Write(ret.Marshal())
		return
	}

	data, err := jsoniter.Marshal(req)
	if err != nil {
		errMsg := fmt.Sprintf("can't resolve json data: %s", err.Error())
		logger.Warn(errMsg)
		ret := Error(500, errMsg)
		w.Write(ret.Marshal())
		return
	}

	if err = h.Ctx.State.ProxyMap.RemoveHost(req.Pattern, req.Host); err != nil {
		w.Write(Error(500, err.Error()).Marshal())
		return
	}

	if err = h.Ctx.writeLogEntry(5, data); err != nil {
		errMsg := fmt.Sprintf("can't apply log entry: %s", err.Error())
		logger.Warn(errMsg)
		ret := Error(500, errMsg)
		w.Write(ret.Marshal())
		return
	}
	w.Write(Ok().Marshal())
	return
}

func (h *HttpServer) doGetBalancerMode(w http.ResponseWriter, r *http.Request) {
	typies := balancer.GetBalancerType()
	w.Write(Ok().Put("mode", typies).Marshal())
}

func (h *HttpServer) doChangeLb(w http.ResponseWriter, r *http.Request) {
	type ReqData struct {
		Prefix string `json:"prefix"`
		Lb     string `json:"lb"`
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

	_, err := jsoniter.Marshal(req)
	if err != nil {
		errMsg := fmt.Sprintf("can't resolve json data: %s", err.Error())
		logger.Warn(errMsg)
		ret := Error(500, errMsg)
		w.Write(ret.Marshal())
		return
	}

	err = h.Ctx.State.ProxyMap.Relations[req.Prefix].ChangeLb(req.Lb)
	if err != nil {
		w.Write(Error(500, err.Error()).Marshal())
		return
	}
	w.Write(Ok().Marshal())
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

func validLocation(location config.Location) error {
	pattern := location.Pattern
	if pattern[0] != '/' {
		return fmt.Errorf("the pattern must begin with character '/'")
	}
	proxyPass := location.ProxyPass
	if len(proxyPass) == 0 {
		return fmt.Errorf("can't find any proxy hosts")
	}
	return nil
}
