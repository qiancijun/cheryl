package balancer

import "errors"

type Algorithm string

// 反向代理的负载均衡器
type Balancer interface {
	Add(string)
	Remove(string)
	Balance(string) (string, error)
	Inc(string)
	Done(string)
	Len() int
}

type Factory func([]string) Balancer

var (
	NoHostError                = errors.New("no host")
	AlgorithmNotSupportedError = errors.New("algorithm not supported")
	factories                  = make(map[Algorithm]Factory)
)

func Build(algo Algorithm, hosts []string) (Balancer, error) {
	factory, ok := factories[algo]
	if !ok {
		return nil, AlgorithmNotSupportedError
	}
	return factory(hosts), nil
}

func GetBalancerType() []string {
	res := make([]string, 0)
	for k := range factories {
		res = append(res, string(k))
	}
	return res
}