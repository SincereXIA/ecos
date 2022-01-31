package config

import (
	"ecos/cloud/sun"
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
	Capacity    uint64
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
		Capacity:    0,
	}
	//DefaultConfig.Capacity = common.GetAvailStorage(DefaultConfig.StoragePath)
}

// InitConfig check config and init data dir and set some empty config value
func InitConfig(conf *Config) error {
	var defaultConfig Config
	_ = config.GetDefaultConf(&defaultConfig)
	// check storagePath
	if conf.StoragePath == "" {
		conf.StoragePath = defaultConfig.StoragePath
	}
	err := common.InitPath(conf.StoragePath)
	if err != nil {
		return err
	}
	conf.Capacity = common.GetAvailStorage(conf.StoragePath)
	if conf.Capacity == 0 {
		return errors.New("cannot write to storage path")
	}

	// read persist config file in storage path
	storagePath := conf.StoragePath
	confPath := path.Join(storagePath + "/config/edge_node.json")
	s, err := os.Stat(confPath)
	if err == nil && !s.IsDir() && s.Size() > 0 {
		config.Register(conf, confPath)
		config.ReadAll()
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
