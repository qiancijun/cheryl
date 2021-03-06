package cheryl

import (
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/hashicorp/raft"
	raftboltdb "github.com/hashicorp/raft-boltdb"
	"github.com/qiancijun/cheryl/config"
	"github.com/qiancijun/cheryl/logger"
	_ "github.com/qiancijun/cheryl/utils"
)

type raftNodeInfo struct {
	Raft           *raft.Raft
	fsm            *FSM
	leaderNotifych chan bool
}

func newRaftTransport(conf *config.CherylConfig) (*raft.NetworkTransport, error) {
	address, err := net.ResolveTCPAddr("tcp", conf.Raft.RaftTCPAddress)
	if err != nil {
		return nil, err
	}
	transport, err := raft.NewTCPTransport(address.String(), address, 3, 10*time.Second, os.Stderr)
	if err != nil {
		return nil, err
	}
	return transport, nil
}

func createRaftNode(conf *config.CherylConfig, ctx *StateContext) (*raftNodeInfo, error) {
	raftConfig := raft.DefaultConfig()
	// raftConfig.LocalID = raft.ServerID(conf.Raft.RaftTCPAddress)
	// ip, err := utils.GetOutBoundIP()
	// if err != nil {
	// 	logger.Warnf("can't get local ip address: %s", err.Error())
	// 	ip = "0.0.0.0"
	// }
	// raftConfig.LocalID = raft.ServerID(fmt.Sprintf("%s:%d", ip, conf.HttpPort))
	raftConfig.LocalID = raft.ServerID(conf.Name)
	// raftConfig.SnapshotInterval = time.Duration(conf.Raft.SnapshotInterval) * time.Second
	raftConfig.SnapshotInterval = time.Duration(conf.Raft.SnapshotInterval) * time.Second
	raftConfig.SnapshotThreshold = conf.Raft.SnapshotThreshold
	leaderNotify := make(chan bool, 1)
	raftConfig.NotifyCh = leaderNotify
	raftConfig.Logger = nil
	raftConfig.LogLevel = conf.Raft.LogLevel
	raftConfig.HeartbeatTimeout = time.Duration(conf.Raft.HeartbeatTimeout) * time.Second
	raftConfig.ElectionTimeout = time.Duration(conf.Raft.ElectionTimeout) * time.Second

	transport, err := newRaftTransport(conf)
	if err != nil {
		return nil, err
	}
	path := filepath.Join(conf.Raft.DataDir, conf.Name)
	logger.Debugf("create data dir: %s", path)
	if err := os.MkdirAll(path, 0700); err != nil {
		return nil, err
	}

	fsm := &FSM{
		ctx: ctx,
		log: log.New(os.Stdout, "FSM: ", log.Ldate|log.Ltime),
	}
	snapshotStore, err := raft.NewFileSnapshotStore(path, 1, os.Stderr)
	if err != nil {
		return nil, err
	}

	logStore, err := raftboltdb.NewBoltStore(filepath.Join(path, "raft-log.bolt"))
	if err != nil {
		return nil, err
	}

	stableStore, err := raftboltdb.NewBoltStore(filepath.Join(path, "raft-stable.bolt"))
	if err != nil {
		return nil, err
	}

	raftNode, err := raft.NewRaft(raftConfig, fsm, logStore, stableStore, snapshotStore, transport)
	if err != nil {
		return nil, err
	}
	if conf.Raft.IsLeader {
		configuration := raft.Configuration{
			Servers: []raft.Server{
				{
					ID:      raftConfig.LocalID,
					Address: transport.LocalAddr(),
				},
			},
		}
		raftNode.BootstrapCluster(configuration)
	}
	return &raftNodeInfo{
		Raft:           raftNode,
		fsm:            fsm,
		leaderNotifych: leaderNotify,
	}, nil
}

func joinRaftCluster(conf *config.CherylConfig) error {
	url := fmt.Sprintf("http://%s/join?peerAddress=%s&name=%s", conf.Raft.LeaderAddress, conf.Raft.RaftTCPAddress, conf.Name)
	response, err := http.Get(url)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return err
	}
	logger.Debug(string(body))
	if string(body) != "ok" {
		return fmt.Errorf("join cluster fail")
	}
	return nil
}
