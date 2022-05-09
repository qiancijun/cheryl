package config

import (
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
	"strings"

	"com.cheryl/cheryl/balancer"
	"com.cheryl/cheryl/logger"
	reverseproxy "com.cheryl/cheryl/reverse_proxy"
	"gopkg.in/yaml.v3"
)

var (
	ascii = `
	 _____   _   _   _____   _____   __    __  _      
	/  ___| | | | | | ____| |  _  \  \ \  / / | |     
	| |     | |_| | | |__   | |_| |   \ \/ /  | |     
	| |     |  _  | |  __|  |  _  /    \  /   | |     
	| |___  | | | | | |___  | | \ \    / /    | |___  
	\_____| |_| |_| |_____| |_|  \_\  /_/     |_____| `
)

type Config struct {
	SSLCertificateKey string      `yaml:"ssl_certificate_key"`
	Location          []*Location `yaml:"location"`
	Schema            string      `yaml:"schema"`
	Port              int         `yaml:"port"`
	SSLCertificate    string      `yaml:"ssl_certificate"`
	HealthCheck       bool        `yaml:"tcp_health_check"`
	LogLevel          string      `yaml:"log_level"`
	Raft              RaftConfig  `yaml:"raft"`
}

type Location struct {
	Pattern     string   `yaml:"pattern"`
	ProxyPass   []string `yaml:"proxy_pass"`
	BalanceMode string   `yaml:"balance_mode"`
}

type RaftConfig struct {
	DataDir          string `yaml:"data_dir"`
	RaftTCPAddress   string `yaml:"tcp_address"`
	IsLeader         bool   `yaml:"leader"`
	SnapshotInterval int    `yaml:"snapshot_interval"`
}

var config *Config

func ReadConfig(fileName string) (*Config, error) {
	in, err := ioutil.ReadFile(fileName)
	if err != nil {
		return nil, err
	}
	err = yaml.Unmarshal(in, &config)
	if err != nil {
		return nil, err
	}

	switch strings.ToLower(config.LogLevel) {
	case "error":
		logger.LogLevel = 1 << 4
	case "warning":
		logger.LogLevel = 1 << 3
	case "info":
		logger.LogLevel = 1 << 2
	case "debug":
		logger.LogLevel = 1 << 1
	default:
		logger.LogLevel = 1
	}

	return config, nil
}

func (c *Config) Print() {
	fmt.Printf("%s\nSchema: %s\nPort: %d\nLocation:\n", ascii, c.Schema, c.Port)
	for _, l := range c.Location {
		fmt.Printf("\tRoute: %s\n\tProxyPass: %s\n\tMode: %s\n",
			l.Pattern, l.ProxyPass, l.BalanceMode)
	}
}

func (c *Config) Validation() error {
	if c.Schema != "http" && c.Schema != "https" {
		return fmt.Errorf("the schema \"%s\" not supported", c.Schema)
	}
	if len(c.Location) == 0 {
		return errors.New("the details of location cannot be null")
	}
	if c.Schema == "https" && (len(c.SSLCertificate) == 0 || len(c.SSLCertificateKey) == 0) {
		return errors.New("the https proxy requires ssl_certificate_key and ssl_certificate")
	}
	return nil
}

func (c *Config) StartServer() {
	r := http.NewServeMux()

	router := reverseproxy.GetRouterInstance("default")
	hasStaticRouter := false
	for _, l := range c.Location {
		if l.Pattern == "/" {
			hasStaticRouter = true
		}
		httpProxy, err := reverseproxy.NewHTTPProxy(l.Pattern, l.ProxyPass, balancer.Algorithm(l.BalanceMode))
		router.Add(l.Pattern, httpProxy)
		if err != nil {
			log.Fatalf("create proxy error: %s", err)
		}
		if c.HealthCheck {
			httpProxy.HealthCheck()
		}
		r.Handle(l.Pattern, httpProxy)
	}
	if !hasStaticRouter {
		// TODO 没有指定默认路由，将接受到的请求做路由匹配处理
		r.Handle("/", router)
	}

	svr := http.Server{
		Addr:    ":" + strconv.Itoa(c.Port),
		Handler: r,
	}
	c.Print()
	if c.Schema == "http" {
		if err := svr.ListenAndServe(); err != nil {
			log.Fatalf("listen and serve error: %s", err)
		}
	} else if c.Schema == "https" {
		if err := svr.ListenAndServeTLS(c.SSLCertificate, c.SSLCertificateKey); err != nil {
			log.Fatalf("listen and serve error: %s", err)
		}
	}
}

func GetConfig() *Config {
	return config
}
