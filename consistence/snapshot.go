package consistence

import (
	reverseproxy "com.cheryl/cheryl/reverse_proxy"
	"github.com/hashicorp/raft"
)

type snapshot struct {
	proxyMap *reverseproxy.ProxyMap
}

func (s *snapshot) Persist(sink raft.SnapshotSink) error {
	snapshotBytes, err := s.proxyMap.Marshal()
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