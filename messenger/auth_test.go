package messenger

import (
	"context"
	"ecos/messenger/auth"
	config2 "ecos/messenger/config"
	"ecos/messenger/demo"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"testing"
	"time"
)

func TestBasicAuthFunc(t *testing.T) {
	config := config2.DefaultConfig
	t.Run("test Auth Enabled", func(t *testing.T) {
		config.AuthEnabled = true
		config.ListenAddr = "127.0.0.1"
		rpcServer := NewRpcServerWithConfig(config)
		go runTestAuthServer(t, rpcServer)
		time.Sleep(time.Millisecond * 100)
		t.Run("test auth enabled no token ", func(t *testing.T) {
			conn, err := GetRpcConn("localhost", rpcServer.ListenPort)
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
					} else {
						t.Errorf("Error calling SayHello: %s", err)
					}
				}
			} else {
				t.Errorf("Got unexpected reply: %s", reply.Message)
			}
		})
		t.Run("test auth enabled with token ", func(t *testing.T) {
			conn, err := GetRpcConn("localhost", rpcServer.ListenPort)
			if err != nil {
				t.Errorf("Error connecting to server: %s", err)
			}
			c := demo.NewGreeterClient(conn)
			ctx := auth.NewCtxWithToken(context.Background(), "root", "root")
			reply, err := c.SayHello(ctx, &demo.HelloRequest{Name: "world"})
			if err != nil {
				t.Errorf("Error calling SayHello: %s", err)
			} else {
				t.Logf("Got reply: %s", reply.Message)
			}
		})
		rpcServer.Stop()
	})

	t.Run("test Auth Disabled", func(t *testing.T) {
		config.AuthEnabled = false
		rpcServer := NewRpcServerWithConfig(config)
		go runTestAuthServer(t, rpcServer)
		time.Sleep(time.Millisecond * 100)
		t.Run("test auth disabled no token ", func(t *testing.T) {
			conn, err := GetRpcConn("localhost", rpcServer.ListenPort)
			if err != nil {
				t.Errorf("Error connecting to server: %s", err)
			}
			c := demo.NewGreeterClient(conn)
			reply, err := c.SayHello(context.Background(), &demo.HelloRequest{Name: "world"})
			if err != nil {
				st, ok := status.FromError(err)
				if st != nil && ok {
					t.Errorf("Error calling SayHello: %s", err)
				}
			} else {
				t.Logf("Got expected reply: %s", reply.Message)
			}
		})
		rpcServer.Stop()
	})

}

func runTestAuthServer(t *testing.T, rpc *RpcServer) {
	server := demo.Server{}
	demo.RegisterGreeterServer(rpc.Server, &server)
	err := rpc.Run()
	if err != nil {
		t.Errorf("Error running server: %s", err)
	}
}
