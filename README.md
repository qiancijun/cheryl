[![License](https://img.shields.io/github/license/qiancijun/cheryl)](https://opensource.org/licenses/MIT)
![](https://qiancijun-images.oss-cn-beijing.aliyuncs.com/Cheryl.jpg?versionId=CAEQJhiBgICngf6IiRgiIGJkODVjOWExODEyZTQ1OTNhYjUwMTExYzNjZGY1ZTRk)

# 介绍
Cheryl 是由 Golang 编写的一款分布式微服务网关
## 下载
```
go get github.com/qiancijun/cheryl
```
# 快速开始
## 配置文件
``` yaml
name: node1
schema: http
port: 80
http_port: 9119
ssl_certificate:
ssl_certificate_key:
tcp_health_check: true
log_level: error
router_type: default
read_header_timeout: 10
read_timeout: 10
idle_timeout: 10
raft:
  data_dir: ./data
  tcp_address: 127.0.0.1:7000
  leader: true
  leader_address: 127.0.0.1:9119
  snapshot_interval: 20
  snapshot_threshold: 1
  log_level: info
  heartbeat_timeout: 10
  election_timeout: 10
location:                         # route matching for reverse proxy
  - pattern: /api
    proxy_pass:                   # URL of the reverse proxy
    - "http://localhost:8080"
    - "http://localhost:8081"
    # - "http://my-server.com"
    balance_mode: round-robin     # load balancing algorithm
```

## Demo
``` golang
package main

import (
	"github.com/qiancijun/cheryl/config"
	"github.com/qiancijun/cheryl/cheryl"
)

func main() {
	conf, err := config.ReadConfig("./config.yaml")
	if err != nil {
		panic(err)
	}
	cheryl.Start(conf)
}
```
启动成功后，访问9119端口查看WebUI页面