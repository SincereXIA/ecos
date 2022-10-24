package config

type CloudConfig struct {
	BasePath  string
	ChunkSize uint64
}

var DefaultCloudConfig CloudConfig

func init() {
	DefaultCloudConfig = CloudConfig{BasePath: "./ecos-data/cloud/", ChunkSize: 1 << 20}
}
