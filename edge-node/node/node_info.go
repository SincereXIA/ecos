package node

import (
	"github.com/google/uuid"
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
