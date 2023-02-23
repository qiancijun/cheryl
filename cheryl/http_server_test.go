package cheryl

import (
	"io/ioutil"
	"net/http"
	"testing"

	"github.com/qiancijun/cheryl/config"
	"github.com/qiancijun/cheryl/logger"
	reverseproxy "github.com/qiancijun/cheryl/reverse_proxy"
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

func TestGetProxy(t *testing.T) {
	reverseproxy.GetRouterInstance("default")
	m := reverseproxy.NewProxyMap()
	m.AddProxyWithLocation(config.Location{
		Pattern: "/api",
		ProxyPass: []string{
			"http://localhost:8080",
			"http://localhost:8081",
			"http://localhost:8082",
		},
		BalanceMode: "round-robin",
	})
	type host struct {
		Host  string `json:"host"`
		Alive bool   `json:"alive"`
	}
	type Response struct {
		BalancerMode string `json:"balancerMode"`
		Hosts        []host `json:"hosts"`
	}
	data := make(map[string]Response)
	for k, v := range m.Relations {
		proxy := Response{}
		proxy.BalancerMode = v.Lb.Mode()
		proxy.Hosts = make([]host, 0)
		for h := range v.HostMap {
			proxy.Hosts = append(proxy.Hosts, host{h, v.Alive[h]})
		}
		data[k] = proxy
	}
	logger.Debug(data)
}
