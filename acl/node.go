package acl

type Edge struct {
	label string
	next *Node
}

type Node struct {
	isLeaf bool
	edges map[byte]Edge
}

func NewNode(isLeaf bool) *Node {
	return &Node {
		isLeaf: isLeaf,
		edges: make(map[byte]Edge),
	}
}

func NewEdge(label string) Edge {
	return Edge{
		label: label,
		next: NewNode(true),
	}
}

func (n *Node) GetTransition(transitionChar byte) (Edge, bool) {
	if edge, has := n.edges[transitionChar]; has {
		return edge, true
	} else {
		return Edge{}, false
	}
}

func (n *Node) AddEdge(label string, next *Node) {
	n.edges[label[0]] = Edge{label, next}
}

func (n *Node) TotalEdges() int {
	return len(n.edges)
}