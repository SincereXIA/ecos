package demo

import (
	"context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"testing"
	"time"
)

func TestServer_SayHello(t *testing.T) {
	go ServerRun()
	time.Sleep(100 * time.Millisecond)
	conn, err := grpc.Dial(":32671", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Errorf("faild to connect: %v", err)
	}
	defer func(conn *grpc.ClientConn) {
		err = conn.Close()
		if err != nil {
			t.Errorf("close grpc conn err: %v", err)
		}
	}(conn)

	c := NewGreeterClient(conn)
	// 调用服务端的SayHello
	r, err := c.SayHello(context.Background(), &HelloRequest{Name: "ecos"})
	if err != nil {
		t.Errorf("could not greet: %v", err)
	}
	t.Logf("Greeting: %s !\n", r.Message)
}
