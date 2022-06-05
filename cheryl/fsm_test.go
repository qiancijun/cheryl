package cheryl

import (
	"encoding/binary"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLogEntryEncode(t *testing.T) {
	l := LogEntry{
		Opt:  1,
		Data: []byte("hello"),
	}
	data, err := l.Encode()
	assert.NoError(t, err)
	opt := binary.BigEndian.Uint16(data)
	data = data[2:]
	str := string(data)
	assert.Equal(t, uint16(1), opt)
	assert.Equal(t, "hello", str)
}
