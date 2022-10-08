package rainbow

import (
	"ecos/messenger"
	"testing"
)

func TestRainbow(t *testing.T) {
	port, rpcServer := messenger.NewRandomPortRpcServer()
	NewRainbow(rpcServer)

	go rpcServer.Run()

	conn, err := messenger.GetRpcConn("127.0.0.1", port)
	if err != nil {
		t.Fatal(err)
	}

	_ = NewRainbowClient(conn)

}
