package config

import (
	"ecos/utils/config"
	"testing"
)

func TestEdgeNodeConfig(t *testing.T) {
	confPath := "./edge_node.json"
	config.Register(DefaultConfig, confPath)
	config.Write(DefaultConfig)
}
