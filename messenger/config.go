package messenger

type Config struct {
	// The address of the server.
	ListenAddr  string
	ListenPort  int
	AuthEnabled bool
}

var DefaultConfig = Config{
	AuthEnabled: false,
	ListenPort:  0,
	ListenAddr:  "",
}
