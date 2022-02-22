package config

import (
	"ecos/utils/config"
	"os"
)

const blockSize = 1 << 22
const chunkSize = 1 << 20
const uploadTimeoutMs = 1000

type ObjectConfig struct {
	BlockSize uint64
	ChunkSize uint64
}

type Config struct {
	config.Config
	Object          ObjectConfig
	UploadTimeoutMs int
}

var DefaultConfig *Config

func init() {
	DefaultConfig = &Config{
		Config: config.Config{},
		Object: ObjectConfig{
			BlockSize: blockSize,
			ChunkSize: chunkSize,
		},
		UploadTimeoutMs: uploadTimeoutMs,
	}
}

// InitConfig check config and init data dir and set some empty config value
func InitConfig(conf *Config) error {
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
