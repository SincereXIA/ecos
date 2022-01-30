package config

import (
	"ecos/cloud/sun"
	"ecos/edge-node/node"
	"ecos/utils/config"
	"github.com/google/uuid"
	"net"
)

const rpcPort = 3267
const httpPort = 3268

type MoonConf struct {
	SunAddr   string
	GroupInfo sun.GroupInfo
}

type Config struct {
	config.Config
	SelfInfo    *node.NodeInfo
	RpcPort     uint64
	HttpPort    uint64
	Moon        MoonConf
	StoragePath string
}

var DefaultConfig *Config

func init() {
	_, ipAddr := getSelfIpAddr()
	DefaultConfig = &Config{
		Config: config.Config{},
		SelfInfo: &node.NodeInfo{
			RaftId:   1,
			Uuid:     uuid.New().String(),
			IpAddr:   ipAddr,
			RpcPort:  rpcPort,
			Capacity: 0,
		},
		RpcPort:     rpcPort,
		HttpPort:    httpPort,
		Moon:        MoonConf{SunAddr: ""},
		StoragePath: "./ecos-data",
	}
}

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
