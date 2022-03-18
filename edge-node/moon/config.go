package moon

import (
	"ecos/edge-node/node"
	"time"
)

type Config struct {
	SunAddr                string
	GroupInfo              node.GroupInfo
	NodeInfoCommitInterval time.Duration
}

var DefaultConfig *Config

func init() {
	DefaultConfig = &Config{
		SunAddr:                "",
		GroupInfo:              node.GroupInfo{},
		NodeInfoCommitInterval: time.Second,
	}
}
