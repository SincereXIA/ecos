package router

import "ecos/client/config"

type Config struct {
	Host         string
	Port         uint64
	EnableAuth   bool
	ClientConfig config.ClientConfig
}

var DefaultConfig Config

func init() {
	DefaultConfig.Host = "localhost"
	DefaultConfig.Port = 3267
	DefaultConfig.EnableAuth = false
	DefaultConfig.ClientConfig = config.DefaultConfig
}
