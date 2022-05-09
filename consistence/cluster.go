package consistence

import (
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"com.cheryl/cheryl/config"
	"github.com/hashicorp/raft"
	raftboltdb "github.com/hashicorp/raft-boltdb"
)

type raftNodeInfo struct {
	Raft           *raft.Raft
	fsm            *FSM
	leaderNotifych chan bool
}

func newRaftTransport(conf *config.Config) (*raft.NetworkTransport, error) {
	address, err := net.ResolveTCPAddr("tcp", conf.Raft.RaftTCPAddress)
	if err != nil {
		return nil, err
	}
	transport, err := raft.NewTCPTransport(address.String(), address, 3, 10 * time.Second, os.Stderr)
	if err != nil {
		return nil, err
	}
	return transport, nil
}

func Make(conf *config.Config, ctx *StateContext) (*raftNodeInfo, error) {
	raftConfig := raft.DefaultConfig()
	raftConfig.LocalID = raft.ServerID(conf.Raft.RaftTCPAddress)
	raftConfig.SnapshotInterval = time.Duration(conf.Raft.SnapshotInterval) * time.Second
	raftConfig.SnapshotThreshold = 2
	leaderNotify := make(chan bool, 1)
	raftConfig.NotifyCh = leaderNotify

	transport, err := newRaftTransport(conf)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(conf.Raft.DataDir, 0700); err != nil {
		return nil, err
	}

	fsm := &FSM {
		ctx: ctx,
		log: log.New(os.Stdout, "FSM: ", log.Ldate|log.Ltime),
	}
	snapshotStore, err := raft.NewFileSnapshotStore(conf.Raft.DataDir, 1, os.Stderr)
	if err != nil {
		return nil, err
	}

	logStore, err := raftboltdb.NewBoltStore(filepath.Join(conf.Raft.DataDir, "raft-log.bolt"))
	if err != nil {
		return nil, err
	}

	stableStore, err := raftboltdb.NewBoltStore(filepath.Join(conf.Raft.DataDir, "raft-stable.bolt"))
	if err != nil {
		return nil, err
	}

	raftNode, err := raft.NewRaft(raftConfig, fsm, logStore, stableStore, snapshotStore, transport)
	if err != nil {
		return nil, err
	}
	if conf.Raft.IsLeader {
		configuration := raft.Configuration {
			Servers: []raft.Server{
				{
					ID: raftConfig.LocalID,
					Address: transport.LocalAddr(),
				},
			},
		}
		raftNode.BootstrapCluster(configuration)
	}
	return &raftNodeInfo{
		Raft: raftNode,
		fsm: fsm,
		leaderNotifych: leaderNotify,
	}, nil
}

func JoinRaftCluster(conf *config.Config) error {
	url := fmt.Sprintf("http://%s/join?peerAddress=%s", conf.Raft.LeaderAddress, conf.Raft.RaftTCPAddress)
	response, err := http.Get(url)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return err
	}
	if string(body) != "ok" {
		return fmt.Errorf("join cluster fail: %s", err.Error())
	}
	return nil
}