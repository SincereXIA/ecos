package config

import (
	"ecos/edge-node/gaia"
	"ecos/edge-node/moon"
	"ecos/edge-node/node"
	"ecos/utils/common"
	"ecos/utils/config"
	"errors"
	"github.com/google/uuid"
	"net"
	"os"
	"path"
)

const rpcPort = 3267
const httpPort = 3268

type Config struct {
	config.Config
	SelfInfo    *node.NodeInfo
	RpcPort     uint64
	HttpPort    uint64
	StoragePath string

	MoonConfig *moon.Config
	GaiaConfig *gaia.Config
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
			Capacity: 10,
		},
		RpcPort:     rpcPort,
		HttpPort:    httpPort,
		MoonConfig:  moon.DefaultConfig,
		StoragePath: "./ecos-data",
		GaiaConfig:  gaia.DefaultConfig,
	}
	//DefaultConfig.Capacity = common.GetAvailStorage(DefaultConfig.StoragePath)
}

// InitConfig check config and init data dir and set some empty config value
func InitConfig(conf *Config) error {
	var defaultConfig Config
	_ = config.GetDefaultConf(&defaultConfig)

	// check ip address
	if !isAvailableIpAdder(conf.SelfInfo.IpAddr) {
		conf.SelfInfo.IpAddr = defaultConfig.SelfInfo.IpAddr
	}

	// check storagePath
	if conf.StoragePath == "" {
		conf.StoragePath = defaultConfig.StoragePath
	}
	err := common.InitPath(conf.StoragePath)
	if err != nil {
		return err
	}
	conf.SelfInfo.Capacity = common.GetAvailStorage(conf.StoragePath)
	if conf.SelfInfo.Capacity == 0 {
		return errors.New("cannot write to storage path")
	}

	// read persist config file in storage path
	storagePath := conf.StoragePath
	confPath := path.Join(storagePath + "/config/edge_node.json")
	s, err := os.Stat(confPath)
	var persistConf Config
	if err == nil && !s.IsDir() && s.Size() > 0 {
		config.Read(confPath, &persistConf)
		conf.SelfInfo.Uuid = persistConf.SelfInfo.Uuid
	}
	// save persist config file in storage path
	err = config.WriteToPath(conf, confPath)
	return err
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

func isAvailableIpAdder(addr string) bool {
	if addr == "" {
		return false
	}
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return false
	}
	for _, address := range addrs {
		// 检查ip地址判断是否回环地址
		if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.String() == addr {
				return true
			}
		}
	}
	return false
}
