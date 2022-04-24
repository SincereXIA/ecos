package moon

import (
	"ecos/edge-node/infos"
)

type Config struct {
	ClusterInfo        infos.ClusterInfo
	RaftStoragePath    string
	RocksdbStoragePath string
}

var DefaultConfig Config

func init() {
	DefaultConfig = Config{
		ClusterInfo:        infos.ClusterInfo{},
		RaftStoragePath:    "./ecos-data/moon/raft/",
		RocksdbStoragePath: "./ecos-data/moon/rocksdb/",
	}
}
