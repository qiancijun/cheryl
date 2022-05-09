package consistence

import "github.com/hashicorp/raft"

type snapshot struct {
	state *State
}

func (s *snapshot) Persist(sink raft.SnapshotSink) error {
	snapshotBytes, err := s.state.ProxyMap.Marshal()
	if err != nil {
		sink.Cancel()
		return err
	}

	if _, err := sink.Write(snapshotBytes); err != nil {
		sink.Cancel()
		return err
	}
	if err := sink.Close(); err != nil {
		sink.Cancel()
		return err
	}
	return nil
}

func (f *snapshot) Release() {}