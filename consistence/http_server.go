package consistence

import (
	"fmt"
	"net/http"

	"com.cheryl/cheryl/logger"
	"github.com/hashicorp/raft"
)

const (
	ENABLE_WRITE_TRUE  = int32(1)
	ENABLE_WRITE_FALSE = int32(0)
)

type HttpServer struct {
	Mux         *http.ServeMux
	Ctx         *StateContext
	enableWrite int32
}

func NewHttpServer(ctx *StateContext) *HttpServer {
	mux := http.NewServeMux()
	s := &HttpServer{
		Ctx:         ctx,
		Mux:         mux,
		enableWrite: ENABLE_WRITE_FALSE,
	}
	mux.HandleFunc("/ping", s.doPing)
	mux.HandleFunc("/join", s.doJoin)
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
	addPeerFuture := h.Ctx.Ctx.RaftNode.Raft.AddVoter(raft.ServerID(peerAddress), raft.ServerAddress(peerAddress), 0, 0)
	if err := addPeerFuture.Error(); err != nil {
		errMsg := fmt.Sprintf("Error joining peer to raft, peeraddress:%s, err:%v, code:%d", peerAddress, err, http.StatusInternalServerError)
		logger.Warn(errMsg)
		fmt.Fprint(w, errMsg)
		return
	}
	fmt.Fprint(w, "ok")
}

func (h *HttpServer) doPing(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, "pong")
}