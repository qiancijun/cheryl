package acl

import (
	"errors"
	"io"
	"strconv"
	"strings"
	"sync"

	"github.com/qiancijun/cheryl/logger"
	"github.com/qiancijun/cheryl/utils"
	jsoniter "github.com/json-iterator/go"
)

const (
	MAX_IPV4_BIT uint32 = 0x80000000
	NO_VALUE     string = ""
)

var (
	InvaildIpAddress = errors.New("invaild ip address, maybe with out mask")
	InvaildNetMask   = errors.New("invaild net mask, must in [0, 32]")
	CantFindIpNet    = errors.New("can't find ip")
	AccessControlList *RadixTree
)

type RadixTree struct {
	sync.RWMutex
	root   *radixNode
	free   *radixNode
	Record map[string]bool `json:"record"`
}

type radixNode struct {
	right  *radixNode
	left   *radixNode
	parent *radixNode
	value  string
}

func init() {
	AccessControlList = NewRadixTree()
}

func NewRadixTree() *RadixTree {
	logger.Debug("init RadixTree success")
	return &RadixTree{
		root:   &radixNode{nil, nil, nil, NO_VALUE},
		free:   nil,
		Record: make(map[string]bool),
	}
}

func (tree *RadixTree) newNode() *radixNode {
	var node *radixNode
	if tree.free != nil {
		node = tree.free
		tree.free = tree.free.right
		return node
	}
	return &radixNode{nil, nil, nil, NO_VALUE}
}

func (tree *RadixTree) insert(key uint32, mask uint32, value string) {
	var node, next *radixNode
	node, next = tree.root, tree.root
	bit := MAX_IPV4_BIT
	for (bit & mask) != 0 {
		if (key & bit) != 0 {
			next = node.right
		} else {
			next = node.left
		}

		if next == nil {
			break
		}
		bit >>= 1
		node = next
	}

	if next != nil {
		node.value = value
		return
	}

	for (bit & mask) != 0 {
		next = tree.newNode()
		if next == nil {
			logger.Error("can't create radixTree Node, this is a bug")
		}
		next.parent = node
		next.value = NO_VALUE
		if (key & bit) != 0 {
			node.right = next
		} else {
			node.left = next
		}
		bit >>= 1
		node = next
	}
	node.value = value
}

func (tree *RadixTree) Add(ipNet string, value string) error {
	strs := strings.Split(ipNet, "/")
	ipStr := strs[0]
	ip, err := utils.InetToi(ipStr)
	if err != nil {
		return InvaildIpAddress
	}
	if len(strs) < 2 {
		return InvaildIpAddress
	}
	mask := strs[1]
	cidr, err := strconv.Atoi(mask)
	if err != nil {
		return err
	}
	var netMask uint32 = ((1 << (32 - cidr)) - 1) ^ 0xffffffff
	tree.insert(ip, netMask, value)
	tree.Record[ipNet] = true
	return nil
}

func (tree *RadixTree) search(key uint32) string {
	bit := MAX_IPV4_BIT
	node := tree.root
	value := NO_VALUE
	for node != nil {
		if node.value != NO_VALUE {
			value = node.value
		}

		if (key & bit) != 0 {
			node = node.right
		} else {
			node = node.left
		}
		bit >>= 1
	}
	return value
}

func (tree *RadixTree) Search(ip string) string {
	i, _ := utils.InetToi(ip)
	return tree.search(i)
}

func (tree *RadixTree) Delete(ipNet string) error {
	strs := strings.Split(ipNet, "/")
	ipStr := strs[0]
	ip, err := utils.InetToi(ipStr)
	if err != nil || len(strs) < 2 {
		return InvaildIpAddress
	}
	mask := strs[1]
	cidr, err := strconv.Atoi(mask)
	if err != nil || cidr < 0 || cidr > 32 {
		return InvaildNetMask
	}
	var netMask uint32 = ((1 << (32 - cidr)) - 1) ^ 0xffffffff
	ret := tree.delete(ip, netMask)
	if ret {
		delete(tree.Record, ipNet)
		logger.Debugf("{ACL} delete ip address %s success", ipNet)
		return nil
	}
	logger.Debugf("{ACL} can't find ip address %s", ipNet)
	return CantFindIpNet
}

func (tree *RadixTree) delete(key uint32, mask uint32) bool {
	bit := MAX_IPV4_BIT
	node := tree.root
	for node != nil && (bit&mask) != 0 {
		if (key & bit) != 0 {
			node = node.right
		} else {
			node = node.left
		}
		bit >>= 1
	}
	if node == nil {
		return false
	}
	if node.right != nil || node.left != nil {
		if node.value != NO_VALUE {
			node.value = NO_VALUE
			return true
		}
		return false
	}
	for {
		if node.parent.right == node {
			node.parent.right = nil
		} else {
			node.parent.left = nil
		}
		node.right = tree.free
		tree.free = node
		node = node.parent

		if node.right != nil || node.left != nil {
			break
		}
		if node.value != NO_VALUE {
			break
		}
		if node.parent == nil {
			break
		}
	}
	return true
}

func (tree *RadixTree) GetBlackList() []string {
	res := make([]string, 0)
	for k := range tree.Record {
		res = append(res, k)
	}
	return res
}

func (tree *RadixTree) AccessControl(ipAddress string) bool {
	ipWithPort := strings.Split(ipAddress, ":")
	ip := ipWithPort[0]
	logger.Debugf("%s will access the system", ip)
	ret := tree.Search(ip) != ""
	if ret {
		logger.Debugf("%s is forbidden to access system", ip)
	}
	return ret
}


func (tree *RadixTree) Marshal() ([]byte, error) {
	tree.RLock()
	defer tree.RUnlock()
	res, err := jsoniter.Marshal(tree)
	return res, err
}

func (tree *RadixTree) UnMarshal(serialized io.ReadCloser) error {
	if err := jsoniter.NewDecoder(serialized).Decode(&tree); err != nil {
		return err
	}
	return nil
}