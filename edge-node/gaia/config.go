package gaia

type Config struct {
	BasePath  string
	ChunkSize uint64
}

var DefaultConfig *Config

func init() {
	DefaultConfig = &Config{BasePath: "./ecos-data/gaia/", ChunkSize: 1 << 20}
}
