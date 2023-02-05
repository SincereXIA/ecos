package config

import (
	"ecos/client/credentials"
	"ecos/utils/config"
	"time"
)

const chunkSize = 1 << 20
const uploadTimeout = 0
const uploadBuffer = 1 << 25

const (
	ConnectEdge = iota
	ConnectCloud
)

type ObjectConfig struct {
	ChunkSize uint64
}

type ClientConfig struct {
	config.Config
	Credential    credentials.Credential
	Object        ObjectConfig
	UploadTimeout time.Duration
	UploadBuffer  uint64
	NodeAddr      string
	NodePort      uint64

	CloudAddr string
	CloudPort uint64

	ConnectType int
}

var DefaultConfig ClientConfig

func init() {
	DefaultConfig = ClientConfig{
		Config:     config.Config{},
		Credential: credentials.New("root", "root"),
		Object: ObjectConfig{
			ChunkSize: chunkSize,
		},
		UploadTimeout: uploadTimeout,
		UploadBuffer:  uploadBuffer,
		NodeAddr:      "ecos-edge-dev.ecos.svc.cluster.local",
		NodePort:      3267,

		CloudAddr:   "ecos-cloud-dev.ecos.svc.cluster.local",
		CloudPort:   3267,
		ConnectType: ConnectEdge,
	}
}
