package cheryl

import (
	"bytes"
	"encoding/binary"
)

type LogEntry struct {
	Opt  uint16
	Data []byte
}

func (l *LogEntry) Encode() ([]byte, error) {
	buf := new(bytes.Buffer)
	if err := binary.Write(buf, binary.BigEndian, l.Opt); err != nil {
		return nil, err
	}
	_, err := buf.Write(l.Data)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (l *LogEntry) Decode(data []byte) error {
	return nil
}