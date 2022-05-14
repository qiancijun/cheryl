package consistence

import (
	"com.cheryl/cheryl/logger"
	jsoniter "github.com/json-iterator/go"
)

type R struct {
	Code int                    `json:code`
	Msg  string                 `json:msg`
	Data map[string]interface{} `json:data`
}

func Ok() *R {
	res := R{
		Code: 200,
		Msg:  "success",
		Data: make(map[string]interface{}),
	}
	return &res
}

func Error(c int, m string) *R {
	res := R{
		Code: c,
		Msg:  m,
		Data: make(map[string]interface{}),
	}
	return &res
}

func (res *R) Put(name string, value interface{}) *R {
	res.Data[name] = value
	return res
}
func (res *R) Marshal() []byte {
	buf, err := jsoniter.Marshal(res)
	if err != nil {
		logger.Warnf("can't marshal R: %s", err.Error())
		return []byte("")
	}
	return buf
}