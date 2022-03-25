package watcher

import (
	"ecos/edge-node/infos"
	"time"
)

type Config struct {
	SunAddr                string
	ClusterInfo            infos.ClusterInfo
	NodeInfoCommitInterval time.Duration
}

var DefaultConfig *Config

func init() {
	DefaultConfig = &Config{
		SunAddr:                "",
		ClusterInfo:            infos.ClusterInfo{},
		NodeInfoCommitInterval: time.Second,
	}
}
