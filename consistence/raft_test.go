package consistence

import (
	"io/ioutil"
	"net/http"
	"testing"

	"com.cheryl/cheryl/config"
	"github.com/stretchr/testify/assert"
)

func TestRaftNode(t *testing.T) {
	config, err := config.ReadConfig("../config.yaml")
	assert.NoError(t, err)
	assert.NotNil(t, config)
	err = config.Validation()
	assert.NoError(t, err)

	go func() {
		Start(config)
	} ()

	url := "http://localhost:9119/ping"
	resp, err := http.Get(url)
	assert.NoError(t, err)
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	assert.Equal(t, "pong", string(body))
}

func TestRaftCluster(t *testing.T) {
	config1, err := config.ReadConfig("../config.yaml")
	assert.NoError(t, err)
	config2, err := config.ReadConfig("../config2.yaml")
	assert.NoError(t, err)
	go func() {
		Start(config1)
	}()
	Start(config2)

}