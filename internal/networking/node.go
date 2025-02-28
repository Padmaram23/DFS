package networking

import "sync"

type Node struct {
	ID       string `json:"id"`
	APIPort  string `json:"api_port"`
	Address  string `json:"address"`
	IsOnline bool   `json:"is_online"`
}

type NodeRegistry struct {
	mu    sync.RWMutex
	nodes map[string]Node
}

func NewNodeRegistry() *NodeRegistry {
	return &NodeRegistry{
		nodes: make(map[string]Node),
	}
}

func (nr *NodeRegistry) RegisterNode(node Node) {
	nr.mu.Lock()
	defer nr.mu.Unlock()
	nr.nodes[node.ID] = node
}

func (nr *NodeRegistry) GetAllNodes() []Node {
	nr.mu.RLock()
	defer nr.mu.RUnlock()
	nodes := make([]Node, 0, len(nr.nodes))
	for _, node := range nr.nodes {
		nodes = append(nodes, node)
	}
	return nodes
}
