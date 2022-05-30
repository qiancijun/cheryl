package config

import (
	"errors"
	"fmt"
	"io/ioutil"
	"strings"

	"com.cheryl/cheryl/logger"
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

type CherylConfig struct {
	Name              string     `yaml:"name"`
	SSLCertificateKey string     `yaml:"ssl_certificate_key"`
	Location          []Location `yaml:"location"`
	Schema            string     `yaml:"schema"`
	Port              int        `yaml:"port"`
	HttpPort          int        `yaml:"http_port"`
	SSLCertificate    string     `yaml:"ssl_certificate"`
	HealthCheck       bool       `yaml:"tcp_health_check"`
	LogLevel          string     `yaml:"log_level"`
	Raft              RaftConfig `yaml:"raft"`
	RouterType        string     `yaml:"router_type"`
}

type Location struct {
	Pattern     string   `yaml:"pattern"`
	ProxyPass   []string `yaml:"proxy_pass"`
	BalanceMode string   `yaml:"balance_mode"`
}

type RaftConfig struct {
	DataDir           string `yaml:"data_dir"`
	RaftTCPAddress    string `yaml:"tcp_address"`
	LeaderAddress     string `yaml:"leader_address"`
	IsLeader          bool   `yaml:"leader"`
	SnapshotInterval  int    `yaml:"snapshot_interval"`
	SnapshotThreshold uint64    `yaml:"snapshot_threshold"`
	LogLevel          string `yaml:"log_level"`
	HeartbeatTimeout  int    `yaml:"heartbeat_timeout"`
	ElectionTimeout   int    `yaml:"election_timeout"`
}

var config *CherylConfig

func ReadConfig(fileName string) (*CherylConfig, error) {
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

func (c *CherylConfig) Print() {
	fmt.Printf("%s\nSchema: %s\nPort: %d\nLocation:\n", ascii, c.Schema, c.Port)
	for _, l := range c.Location {
		fmt.Printf("\tRoute: %s\n\tProxyPass: %s\n\tMode: %s\n",
			l.Pattern, l.ProxyPass, l.BalanceMode)
	}
}

func (c *CherylConfig) Validation() error {
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

func GetConfig() *CherylConfig {
	return config
}
