package consistence

import (
	"fmt"
	"io"
	"log"

	"com.cheryl/cheryl/logger"
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
	opt := e.opt
	switch opt {

	}
	// ret := f.ctx.St.Ca.Set(e.Key, e.Value)
	// f.log.Printf("fms.Apply(), logEntry:%s, ret:%v\n", logEntry.Data, ret)
	// return ret
	return nil
}

// Snapshot returns a latest snapshot
func (f *FSM) Snapshot() (raft.FSMSnapshot, error) {
	return nil, nil
	// return &snapshot{cm: f.ctx.St.Ca}, nil
}

// Restore stores the key-value store to a previous state.
func (f *FSM) Restore(serialized io.ReadCloser) error {
	// return f.ctx.St.Ca.UnMarshal(serialized)
	return nil
}
