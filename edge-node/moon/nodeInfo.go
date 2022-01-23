package moon

import (
	"github.com/google/uuid"
	"net"
	"sync"
)

type NodeInfo struct {
	ID      NodeID
	Uuid    uuid.UUID
	IpAddr  string
	RpcPort uint64
}

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
			Uuid:   uuid.New(),
			IpAddr: ipAddr,
		}
	})
	return selfInfo
}

// NewSelfInfo Generate new nodeInfo
// ** just for test **
func NewSelfInfo(id NodeID, ipaddr string, rpcPort uint64) *NodeInfo {
	selfInfo := &NodeInfo{
		ID:      id,
		Uuid:    uuid.New(),
		IpAddr:  ipaddr,
		RpcPort: rpcPort,
	}
	return selfInfo
}
