package utils

import (
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"
)

var ConnectionTimeout = 2 * time.Second

func GetIP(remoteAddr string) string {
	remoteHost, _, _ := net.SplitHostPort(remoteAddr)
	return remoteHost
}

func GetHost(url *url.URL) string {
	if _, _, err := net.SplitHostPort(url.Host); err == nil {
		return url.Host
	}
	if url.Scheme == "http" {
		return fmt.Sprintf("%s:%s", url.Host, "80")
	} else if url.Scheme == "https" {
		return fmt.Sprintf("%s:%s", url.Host, "443")
	}
	return url.Host
}

// 尝试与目标主机建立连接
func IsBackendAlive(host string) bool {
	addr, err := net.ResolveTCPAddr("tcp", host)
	if err != nil {
		return false
	}
	resolve := fmt.Sprintf("%s:%d", addr.IP, addr.Port)
	conn, err := net.DialTimeout("tcp", resolve, ConnectionTimeout)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

func GetOutBoundIP()(ip string, err error)  {
    conn, err := net.Dial("udp", "8.8.8.8:53")
    if err != nil {
        fmt.Println(err)
        return
    }
    localAddr := conn.LocalAddr().(*net.UDPAddr)
    fmt.Println(localAddr.String())
    ip = strings.Split(localAddr.String(), ":")[0]
    return
}