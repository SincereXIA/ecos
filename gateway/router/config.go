package router

type Config struct {
	Host       string
	Port       uint64
	EnableAuth bool
}

var DefaultConfig Config

func init() {
	DefaultConfig.Host = "localhost"
	DefaultConfig.Port = 3276
	DefaultConfig.EnableAuth = false
}
