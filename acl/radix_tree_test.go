package acl

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRadixTree(t *testing.T) {
	tr := NewRadixTree()
	ip := "192.168.3.1/32"
	tr.Add(ip, ip)
	ip2 := "192.168.3.2/32"
	tr.Add(ip2, ip2)
	ip3 := "192.168.3.3/32"
	tr.Add(ip3, ip3)
	ip4 := "192.168.3.4/32"
	tr.Add(ip4, ip4)

	ans := tr.Search("192.168.3.1")
	assert.Equal(t, ip, ans)
	ans = tr.Search("192.168.3.2")
	assert.Equal(t, ip2, ans)
	ans = tr.Search("192.168.3.3")
	assert.Equal(t, ip3, ans)
	ans = tr.Search("192.168.3.4")
	assert.Equal(t, ip4, ans)

	res := tr.Delete("192.168.3.1/32")
	assert.True(t, res)
	ans = tr.Search("192.168.3.1")
	assert.Equal(t, "", ans)
	ans = tr.Search("192.168.3.2")
	assert.Equal(t, ip2, ans)
	ans = tr.Search("192.168.3.3")
	assert.Equal(t, ip3, ans)
	ans = tr.Search("192.168.3.4")
	assert.Equal(t, ip4, ans)
}
