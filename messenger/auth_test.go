package messenger

import (
	"context"
	"ecos/messenger/demo"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"testing"
	"time"
)

func TestBasicAuthFunc(t *testing.T) {
	config := DefaultConfig
	config.AuthEnabled = true
	rpc := NewRpcServerWithConfig(config)
	go runTestAuthServer(t, rpc)
	time.Sleep(time.Second)
	runDemoClient(t, rpc.ListenPort)
}

func runTestAuthServer(t *testing.T, rpc *RpcServer) {
	server := demo.Server{}
	demo.RegisterGreeterServer(rpc.Server, &server)
	err := rpc.Run()
	if err != nil {
		t.Errorf("Error running server: %s", err)
	}
}

func runDemoClient(t *testing.T, listenPort uint64) {
	conn, err := GetRpcConn("localhost", listenPort)
	if err != nil {
		t.Errorf("Error connecting to server: %s", err)
	}
	c := demo.NewGreeterClient(conn)
	reply, err := c.SayHello(context.Background(), &demo.HelloRequest{Name: "world"})
	if err != nil {
		st, ok := status.FromError(err)
		if st != nil && ok {
			if st.Code() == codes.Unauthenticated {
				t.Logf("Got expected error: %s", err)
				return
			}
		}
		t.Errorf("Error calling SayHello: %s", err)
		return
	}
	t.Errorf("Got unexpected reply: %s", reply.Message)
}
