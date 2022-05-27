package acl

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRadixTree(t *testing.T) {
	radixTree := NewRadixTree()
	radixTree.Insert("test")
	radixTree.Insert("tester")
	radixTree.Insert("water")
	radixTree.Insert("team")
	radixTree.Insert("slow")

	radixTree.PrintAllWords()
	radixTree.Insert("slower")
	ans := radixTree.Search("te")
	assert.Equal(t, false, ans)
	ans = radixTree.Search("test")
	assert.Equal(t, true, ans)
	radixTree.Delete("slow")
	ans = radixTree.Search("slow")
	assert.Equal(t, false, ans)
}

func TestIp(t *testing.T) {
	radixTree := NewRadixTree()
	radixTree.Insert("192.168")
	ip := "192.168.1.1"
	ans := radixTree.IPV4(ip)
	assert.Equal(t, true, ans)
}