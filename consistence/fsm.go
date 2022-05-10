package consistence

import (
	"fmt"
	"io"
	"log"

	"com.cheryl/cheryl/balancer"
	"com.cheryl/cheryl/config"
	"com.cheryl/cheryl/logger"
	reverseproxy "com.cheryl/cheryl/reverse_proxy"
	"github.com/hashicorp/raft"
	jsoniter "github.com/json-iterator/go"
)

const ()

type FSM struct {
	ctx *StateContext
	log *log.Logger
}

/**
opt: 操作类型：

*/
type LogEntryData struct {
	opt   int
	key   string
	value interface{}
}

// Apply applies a Raft log entry to the key-value store.
func (f *FSM) Apply(logEntry *raft.Log) interface{} {
	e := LogEntryData{}

	if err := jsoniter.Unmarshal(logEntry.Data, &e); err != nil {
		errMsg := fmt.Sprintf("Failed unmarshaling Raft log entry. This is a bug. %s", err.Error())
		logger.Error(errMsg)
		panic(errMsg)
	}
	var ret interface{}
	opt := e.opt
	switch opt {
	case 1:
		ret = f.doNewHttpProxy(e)
	}
	// ret := f.ctx.St.Ca.Set(e.Key, e.Value)
	// f.log.Printf("fms.Apply(), logEntry:%s, ret:%v\n", logEntry.Data, ret)
	// return ret
	return ret
}

// Snapshot returns a latest snapshot
func (f *FSM) Snapshot() (raft.FSMSnapshot, error) {
	return &snapshot{proxyMap: &f.ctx.State.ProxyMap}, nil
}

// Restore stores the key-value store to a previous state.
func (f *FSM) Restore(serialized io.ReadCloser) error {
	return f.ctx.State.ProxyMap.UnMarshal(serialized)
}


func (f *FSM) doNewHttpProxy(logEntry LogEntryData) error {
	key, value := logEntry.key, logEntry.value.(*config.Location)
	httpProxy, err := reverseproxy.NewHTTPProxy(value.Pattern, value.ProxyPass, balancer.Algorithm(value.BalanceMode))
	if err != nil {
		logger.Errorf("create proxy error: %s", err)
		return err
	}
	f.ctx.State.ProxyMap.Router.Add(key, httpProxy)
	httpProxy.HealthCheck()
	return nil
}