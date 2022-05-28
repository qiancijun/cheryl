package utils

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"com.cheryl/cheryl/logger"
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

func InetToi(ip string) (uint32, error) {
	ipSegs := strings.Split(ip, ".")
	var ret uint32 = 0
	pos := 24
	for _, ipSeg := range ipSegs {
		tempInt, err := strconv.Atoi(ipSeg)
		if err != nil {
			logger.Warnf("{inetToi} can't convert ip address: %s", err.Error())
			return 0, err
		}
		tempInt <<= pos
		ret |= uint32(tempInt)
		pos -= 8
	}
	return ret, nil
}

func RemoteIp(req *http.Request) string {
	var remoteAddr string
	// RemoteAddr
	remoteAddr = req.RemoteAddr
	if remoteAddr != "" {
		return remoteAddr
	}
	// ipv4
	remoteAddr = req.Header.Get("ipv4")
	if remoteAddr != "" {
		return remoteAddr
	}
	//
	remoteAddr = req.Header.Get("XForwardedFor")
	if remoteAddr != "" {
		return remoteAddr
	}
	// X-Forwarded-For
	remoteAddr = req.Header.Get("X-Forwarded-For")
	if remoteAddr != "" {
		return remoteAddr
	}
	// X-Real-Ip
	remoteAddr = req.Header.Get("X-Real-Ip")
	if remoteAddr != "" {
		return remoteAddr
	} else {
		remoteAddr = "127.0.0.1"
	}
	return remoteAddr
}