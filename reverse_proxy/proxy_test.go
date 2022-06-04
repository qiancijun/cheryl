package reverseproxy

import (
	"testing"

	"github.com/qiancijun/cheryl/config"
	"github.com/stretchr/testify/assert"
)

func init() {
	GetRouterInstance("default")
}

func TestRemoveHost(t *testing.T) {
	m := NewProxyMap()
	err := m.AddProxyWithLocation(config.Location{
		Pattern: "/api",
		ProxyPass: []string{
			"http://localhost:8080",
		},
		BalanceMode: "round-robin",
	})
	assert.NoError(t, err)
	httpProxy := m.Relations["/api"]
	assert.NotNil(t, httpProxy)
	err = m.AddProxy("/api", "http://localhost:8081")
	assert.NoError(t, err)
	err = m.RemoveHost("/api", "localhost:8081")
	assert.NoError(t, err)
	assert.Equal(t, 1, len(httpProxy.HostMap))
}

func TestRemoveProxy(t *testing.T) {
	m := NewProxyMap()
	err := m.AddProxyWithLocation(config.Location{
		Pattern: "/api",
		ProxyPass: []string{
			"http://localhost:8080",
		},
		BalanceMode: "round-robin",
	})
	assert.NoError(t, err)
	err = m.RemoveProxy("/api")
	assert.NoError(t, err)
	api := m.Relations["api"]
	assert.Nil(t, api)
}