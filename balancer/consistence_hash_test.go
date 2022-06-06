package balancer

import (
	"testing"

	"github.com/qiancijun/cheryl/config"
	"github.com/stretchr/testify/assert"
)

func init() {
	config.ReadConfig("../config.yaml")
}

func TestConsistenceHash_Add(t *testing.T) {
	cfg := config.GetConfig()
	cases := []struct {
		name   string
		lb     Balancer
		args   string
		expect int
	}{
		{
			"test-1",
			NewConsistenceHash([]string{
				"http://127.0.0.1:8000",
				"http://127.0.0.1:8001",
				"http://127.0.0.1:8002",
			}),
			"http://127.0.0.1:8003",
			4 * cfg.LoadBalance.Replicas,
		}, {
			"test-1",
			NewConsistenceHash([]string{
				"http://127.0.0.1:8000",
				"http://127.0.0.1:8001",
				"http://127.0.0.1:8002",
			}),
			"http://127.0.0.1:8002",
			3 * cfg.LoadBalance.Replicas,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			c.lb.Add(c.args)
			assert.Equal(t, c.expect, c.lb.Len())
		})
	}
}

func TestConsistenceHashBalance(t *testing.T) {
	ch := NewConsistenceHash([]string{
		"http://127.0.0.1:8000",
		"http://127.0.0.1:8001",
		"http://127.0.0.1:8002",
	})
	ans, _ := ch.Balance("http://127.0.0.1/api/hello")
	ans2, _ := ch.Balance("http://127.0.0.1/api/index")
	cases := []struct {
		name   string
		url    string
		expect string
	}{
		{
			"test-1",
			"http://127.0.0.1/api/hello",
			ans,
		},
		{
			"test-2",
			"http://127.0.0.1/api/index",
			ans2,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ret, _ := ch.Balance(c.url)
			t.Log(ret)
			assert.Equal(t, c.expect, ret)
		})
	}
}
