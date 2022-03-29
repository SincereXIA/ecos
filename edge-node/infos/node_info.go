package infos

import (
	"github.com/google/uuid"
	"github.com/sincerexia/gocrush"
	"strconv"
)

func (m *NodeInfo) GetInfoType() InfoType {
	return InfoType_NODE_INFO
}

func (m *NodeInfo) BaseInfo() *BaseInfo {
	return &BaseInfo{Info: &BaseInfo_NodeInfo{NodeInfo: m}}
}

func (m *NodeInfo) GetID() string {
	return strconv.FormatUint(m.RaftId, 10)
}

// NewSelfInfo Generate new nodeInfo
// ** just for test **
func NewSelfInfo(raftID uint64, ipaddr string, rpcPort uint64) *NodeInfo {
	selfInfo := &NodeInfo{
		RaftId:   raftID,
		Uuid:     uuid.New().String(),
		IpAddr:   ipaddr,
		RpcPort:  rpcPort,
		Capacity: 10,
	}
	return selfInfo
}

type EcosNode struct {
	*NodeInfo
	Type     int
	Failed   bool
	Selector gocrush.Selector
	Root     *EcosNode
	Children []gocrush.Node
}

const (
	RootNodeType int = iota
	LeafNodeType
)

func NewRootNode() *EcosNode {
	// defaultRootNode is used as the default root of implicit condition
	return &EcosNode{
		NodeInfo: &NodeInfo{
			RaftId:  0,
			Uuid:    uuid.New().String(),
			IpAddr:  "0.0.0.0",
			RpcPort: 0,
		},
		Type:   RootNodeType,
		Failed: false,
	}
}

func (x *EcosNode) GetChildren() []gocrush.Node {
	return x.Children
}

func (x *EcosNode) GetType() int {
	return 1
}

func (x *EcosNode) GetWeight() int64 {
	return int64(x.Capacity)
}

func (x *EcosNode) GetId() string {
	return x.GetUuid()
}

func (x *EcosNode) IsFailed() bool {
	return x.Failed
}

func (x *EcosNode) GetSelector() gocrush.Selector {
	return x.Selector
}

func (x *EcosNode) SetSelector(selector gocrush.Selector) {
	x.Selector = selector
}

func (x *EcosNode) GetParent() gocrush.Node {
	if x == x.Root {
		return nil
	} else {
		return x.Root
	}
}

func (x *EcosNode) IsLeaf() bool {
	return x != x.Root
}

func (x *EcosNode) Select(input int64, round int64) gocrush.Node {
	return x.GetSelector().Select(input, round)
}

func (x *EcosNode) GetRaftId() uint64 {
	return x.RaftId
}
