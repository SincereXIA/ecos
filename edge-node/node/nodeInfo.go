package node

import (
	"github.com/google/uuid"
	"net"
	"sync"
)

var selfInfo *NodeInfo
var once sync.Once

func getSelfIpAddr() (error, string) {
	addrs, err := net.InterfaceAddrs()
	selfIp := ""
	if err != nil {
		return err, selfIp
	}
	for _, address := range addrs {
		// 检查ip地址判断是否回环地址
		if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				selfIp = ipnet.IP.String()
			}
		}
	}
	return nil, selfIp
}

func GetSelfInfo() *NodeInfo {
	once.Do(func() {
		_, ipAddr := getSelfIpAddr()
		selfInfo = &NodeInfo{
			RaftId:  0,
			Uuid:    uuid.New().String(),
			IpAddr:  ipAddr,
			RpcPort: 3267, // TODO: set port from config file
		}
	})
	return selfInfo
}

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
