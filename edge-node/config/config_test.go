package config

import (
	"ecos/utils/common"
	"ecos/utils/config"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestEdgeNodeConfig(t *testing.T) {
	confPath := "../edge_node.json"
	config.Register(DefaultConfig, confPath)
	err := config.Write(DefaultConfig)
	if err != nil {
		t.Errorf("write config error: %v", err)
	}
}

func TestGetAvailStorage(t *testing.T) {
	path := "."
	size := common.GetAvailStorage(path)
	t.Logf("Available size: %v", size)
	assert.True(t, size > 0)
}
