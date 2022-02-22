package config

import (
	"ecos/utils/common"
	"ecos/utils/config"
	"github.com/stretchr/testify/assert"
	"os"
	"testing"
)

func TestEdgeNodeConfig(t *testing.T) {
	confPath := "./edge_node.json"
	config.Register(DefaultConfig, confPath)
	err := config.Write(DefaultConfig)
	if err != nil {
		t.Errorf("write config error: %v", err)
	}
	_ = os.Remove(confPath)
}

func TestGetAvailStorage(t *testing.T) {
	path := "."
	size := common.GetAvailStorage(path)
	t.Logf("Available size: %v", size)
	assert.True(t, size > 0)
}
