package cheryl

import (
	"io"

	"github.com/qiancijun/cheryl/acl"
	reverseproxy "github.com/qiancijun/cheryl/reverse_proxy"
	"github.com/hashicorp/raft"
	jsoniter "github.com/json-iterator/go"
)

type snapshot struct {
	ProxyMap *reverseproxy.ProxyMap
	RadixTree *acl.RadixTree
}

func (s *snapshot) Persist(sink raft.SnapshotSink) error {
	snapshotBytes, err := s.Marshal()
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

func (s *snapshot) Marshal() ([]byte, error) {
	res, err := jsoniter.Marshal(s)
	return res, err
}

func (s *snapshot) UnMarshal(serialized io.ReadCloser) error {
	if err := jsoniter.NewDecoder(serialized).Decode(&s); err != nil {
		return err
	}
	return nil
}
