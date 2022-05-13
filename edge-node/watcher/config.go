package watcher

import (
	"ecos/edge-node/infos"
	"time"
)

type Config struct {
	SunAddr                string
	SelfNodeInfo           infos.NodeInfo
	ClusterInfo            infos.ClusterInfo
	NodeInfoCommitInterval time.Duration
	PrometheusAddr         string
	ClusterName            string
}

var DefaultConfig Config

func init() {
	DefaultConfig = Config{
		SunAddr:                "",
		ClusterInfo:            infos.ClusterInfo{},
		SelfNodeInfo:           *infos.NewSelfInfo(1, "127.0.0.1", 0),
		NodeInfoCommitInterval: time.Second * 2,
		PrometheusAddr:         "http://gateway.prometheus.sums.top",
		ClusterName:            "ecos_dev",
	}
}
