package consistence

import (
	"io/ioutil"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewHttpServer(t *testing.T) {
	listener, err := createListener(8080)
	assert.NoError(t, err)
	assert.NotNil(t, listener)

	ctx := &StateContext{}
	h := newHttpServer(ctx)
	go func() {
		http.Serve(listener, h.Mux)
	}()
	url := "http://localhost:8080/ping"
	resp, err := http.Get(url)
	assert.NoError(t, err)
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	assert.NoError(t, err)
	assert.Equal(t, "hello world", string(body))
}