package node

import (
	"github.com/google/uuid"
	"github.com/sincerexia/gocrush"
)

// NewSelfInfo Generate new nodeInfo
// ** just for test **
func NewSelfInfo(raftID uint64, ipaddr string, rpcPort uint64) *NodeInfo {
	selfInfo := &NodeInfo{
		RaftId:  raftID,
		Uuid:    uuid.New().String(),
		IpAddr:  ipaddr,
		RpcPort: rpcPort,
	}
	return selfInfo
}

type Node struct {
	NodeInfo
	Type     int
	Failed   bool
	Selector gocrush.Selector
}

var rootNode = &Node{
	NodeInfo: NodeInfo{
		RaftId:  0,
		Uuid:    uuid.New().String(),
		IpAddr:  "0.0.0.0",
		RpcPort: 0,
	},
	Type:     1,
	Failed:   false,
	Selector: nil,
}

func (x *Node) GetChildren() []gocrush.Node {
	return nil
}

func (x *Node) GetType() int {
	return 1
}

func (x *Node) GetWeight() int64 {
	return int64(x.Capacity)
}

func (x *Node) GetId() string {
	return x.GetUuid()
}

func (x *Node) IsFailed() bool {
	return x.Failed
}

func (x *Node) GetSelector() gocrush.Selector {
	return x.Selector
}

func (x *Node) SetSelector(selector gocrush.Selector) {
	x.Selector = selector
}

func (x *Node) GetParent() gocrush.Node {
	if x == rootNode {
		return nil
	} else {
		return rootNode
	}
}
func (x *Node) IsLeaf() bool {
	return x != rootNode
}

func (x *Node) Select(input int64, round int64) gocrush.Node {
	return x.GetSelector().Select(input, round)
}
