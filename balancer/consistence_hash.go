package balancer

import (
	"errors"
	"hash/crc32"
	"sort"
	"strconv"
	"sync"

	"github.com/qiancijun/cheryl/config"
)

type Hash func(data []byte) uint32

type ConsistenceHash struct {
	hash     Hash
	replicas int
	keys     []int
	hosts    map[int]string
	sync.RWMutex
}

func init() {
	factories["consistence-hash"] = NewConsistenceHash
}

func NewConsistenceHash(hosts []string) Balancer {
	cfg := config.GetConfig()
	ch := &ConsistenceHash{
		replicas: cfg.LoadBalance.Replicas,
		hash:     crc32.ChecksumIEEE,
		hosts:    make(map[int]string),
	}
	for _, host := range hosts {
		ch.Add(host)
	}
	return ch
}

func (c *ConsistenceHash) Add(host string) {
	c.Lock()
	defer c.Unlock()
	for i := 0; i < c.replicas; i++ {
		hash := int(c.hash([]byte(strconv.Itoa(i) + host)))
		c.keys = append(c.keys, hash)
		c.hosts[hash] = host
	}
	sort.Ints(c.keys)
}

func (c *ConsistenceHash) Remove(host string) {
	c.Lock()
	defer c.Unlock()
	for i := 0; i < c.replicas; i++ {
		hash := int(c.hash([]byte(strconv.Itoa(i) + host)))
		idx := sort.SearchInts(c.keys, hash)
		c.keys = append(c.keys[:idx], c.keys[idx+1:]...)
		delete(c.hosts, hash)
	}
}

func (c *ConsistenceHash) Balance(url string) (string, error) {
	c.RLock()
	defer c.RUnlock()
	if len(c.keys) == 0 {
		return "", errors.New("can't find any host")
	}
	hash := int(c.hash([]byte(url)))
	idx := sort.Search(len(c.keys), func(i int) bool {
		return c.keys[i] >= hash
	})
	return c.hosts[c.keys[idx%len(c.keys)]], nil
}

// Inc .
func (c *ConsistenceHash) Inc(_ string) {}

// Done .
func (c *ConsistenceHash) Done(_ string) {}

func (c *ConsistenceHash) Len() int { return len(c.hosts) }