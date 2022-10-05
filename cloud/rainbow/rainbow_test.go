package rainbow

import (
	"context"
	"ecos/messenger"
	"testing"
)

func TestRainbow(t *testing.T) {
	port, rpcServer := messenger.NewRandomPortRpcServer()
	go rpcServer.Run()

	NewRainbow(rpcServer)
	conn, err := messenger.GetRpcConn("127.0.0.1", port)
	if err != nil {
		t.Fatal(err)
	}

	client := NewRainbowClient(conn)

	stream, err := client.GetStream(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 10; i++ {
		err = stream.Send(&Content{
			Payload: &Content_Request{
				Request: &Request{RequestSeq: uint64(i)},
			},
		})

		if err != nil {
			t.Fatal(err)
		}

		content, err := stream.Recv()
		if err != nil {
			t.Fatal(err)
		}

		switch payload := content.Payload.(type) {
		case *Content_Response:
			if payload.Response.ResponseTo != uint64(i) {
				t.Fatal("responseTo not match")
			}
		default:
			t.Fatal("response type not match")
		}
	}

}
