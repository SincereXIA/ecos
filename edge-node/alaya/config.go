package alaya

type Config struct {
	BasePath string
}

var DefaultConfig Config

func init() {
	DefaultConfig = Config{
		BasePath: "./ecos-data/alaya/",
	}
}
