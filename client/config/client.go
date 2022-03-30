package config

import (
	"ecos/utils/config"
	"ecos/utils/logger"
	"os"
	"time"
)

const blockSize = 1 << 22
const chunkSize = 1 << 20
const uploadTimeout = time.Second * 10
const uploadBuffer = 1 << 26
const blockHash = true
const objectHash = false

type ObjectConfig struct {
	BlockSize  uint64
	ChunkSize  uint64
	BlockHash  bool
	ObjectHash bool
}

type ClientConfig struct {
	config.Config
	Object        ObjectConfig
	UploadTimeout time.Duration
	UploadBuffer  uint64
	NodeAddr      string
	NodePort      uint64
}

var DefaultConfig *ClientConfig
var Config *ClientConfig

func init() {
	if DefaultConfig == nil {
		DefaultConfig = &ClientConfig{
			Config: config.Config{},
			Object: ObjectConfig{
				BlockSize:  blockSize,
				ChunkSize:  chunkSize,
				BlockHash:  blockHash,
				ObjectHash: objectHash,
			},
			UploadTimeout: uploadTimeout,
			UploadBuffer:  uploadBuffer,
			NodeAddr:      "ecos-edge-dev",
			NodePort:      3267,
		}
	}
	if Config == nil {
		err := InitConfig(Config)
		if err != nil {
			logger.Warningf("parse client config failed %v", err)
			logger.Warningf("using default client config")
			Config = DefaultConfig
		}
	}
}

// InitConfig check config and init data dir and set some empty config value
func InitConfig(conf *ClientConfig) error {
	// read persist config file in storage path
	// TODO: read confPath from cmd args
	confPath := "./config/client.json"
	s, err := os.Stat(confPath)
	if err == nil && !s.IsDir() && s.Size() > 0 {
		config.Register(DefaultConfig, confPath)
		config.ReadAll()
	}
	err = config.GetConf(conf)
	return err
}
