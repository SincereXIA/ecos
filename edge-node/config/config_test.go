package config

import (
	"ecos/utils/common"
	"ecos/utils/config"
	"go.etcd.io/etcd/Godeps/_workspace/src/github.com/stretchr/testify/assert"
	"testing"
)

func TestEdgeNodeConfig(t *testing.T) {
	confPath := "./edge_node.json"
	config.Register(DefaultConfig, confPath)
	config.Write(DefaultConfig)
}

func TestGetAvailStorage(t *testing.T) {
	path := "/home"
	size := common.GetAvailStorage(path)
	t.Logf("Available size: %v", size)
	assert.True(t, size > 0)
}
