package moon

import (
	"ecos/edge-node/infos"
	"time"
)

type Config struct {
	SunAddr                string
	GroupInfo              infos.GroupInfo
	NodeInfoCommitInterval time.Duration
}

var DefaultConfig *Config

func init() {
	DefaultConfig = &Config{
		SunAddr:                "",
		GroupInfo:              infos.GroupInfo{},
		NodeInfoCommitInterval: time.Second,
	}
}
