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
	"com.cheryl/cheryl/logger"
	"com.cheryl/cheryl/utils"
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
	transport, err := raft.NewTCPTransport(address.String(), address, 3, 10*time.Second, os.Stderr)
	if err != nil {
		return nil, err
	}
	return transport, nil
}

func Make(conf *config.Config, ctx *StateContext) (*raftNodeInfo, error) {
	raftConfig := raft.DefaultConfig()
	// raftConfig.LocalID = raft.ServerID(conf.Raft.RaftTCPAddress)
	ip, err := utils.GetOutBoundIP()
	if err != nil {
		logger.Warnf("can't get local ip address: %s", err.Error())
		ip = "0.0.0.0"
	}
	raftConfig.LocalID = raft.ServerID(fmt.Sprintf("%s:%d", ip, conf.HttpPort))
	// raftConfig.SnapshotInterval = time.Duration(conf.Raft.SnapshotInterval) * time.Second
	raftConfig.SnapshotInterval = 10 * time.Second
	raftConfig.SnapshotThreshold = 2
	leaderNotify := make(chan bool, 1)
	raftConfig.NotifyCh = leaderNotify
	raftConfig.Logger = nil
	raftConfig.HeartbeatTimeout = 5 * time.Second
	raftConfig.ElectionTimeout = 5 * time.Second

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
	logger.Debug(string(body))
	if string(body) != "ok" {
		return fmt.Errorf("join cluster fail: %s", err.Error())
	}
	return nil
}
