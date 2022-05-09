package service

import (
	"net/http"

	"com.cheryl/cheryl/consistence"
)

const (
	ENABLE_WRITE_TRUE  = int32(1)
	ENABLE_WRITE_FALSE = int32(0)
)

type HttpServer struct {
	Mux         *http.ServeMux
	Ctx         *consistence.StateContext
	enableWrite int32
}

func NewHttpServer(ctx *consistence.StateContext) *HttpServer {
	mux := http.NewServeMux()
	s := &HttpServer{
		Ctx:         ctx,
		Mux:         mux,
		enableWrite: ENABLE_WRITE_FALSE,
	}
	return s
}
