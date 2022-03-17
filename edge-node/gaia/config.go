package gaia

type Config struct {
	basePath string
}

var DefaultConfig *Config

func init() {
	DefaultConfig = &Config{basePath: "./ecos-data/gaia/"}
}
