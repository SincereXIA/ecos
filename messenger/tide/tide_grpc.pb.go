// Code generated by protoc-gen-go-grpc. DO NOT EDIT.

package tide

import (
	context "context"
	messenger "ecos/messenger"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
	emptypb "google.golang.org/protobuf/types/known/emptypb"
)

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
// Requires gRPC-Go v1.32.0 or later.
const _ = grpc.SupportPackageIsVersion7

// NodeTideClient is the client API for NodeTide service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type NodeTideClient interface {
	GetClusterStatus(ctx context.Context, in *emptypb.Empty, opts ...grpc.CallOption) (*ClusterStatus, error)
	SendBlock(ctx context.Context, opts ...grpc.CallOption) (NodeTide_SendBlockClient, error)
	SendMetaData(ctx context.Context, in *messenger.ObjectInfo, opts ...grpc.CallOption) (*Result, error)
}

type nodeTideClient struct {
	cc grpc.ClientConnInterface
}

func NewNodeTideClient(cc grpc.ClientConnInterface) NodeTideClient {
	return &nodeTideClient{cc}
}

func (c *nodeTideClient) GetClusterStatus(ctx context.Context, in *emptypb.Empty, opts ...grpc.CallOption) (*ClusterStatus, error) {
	out := new(ClusterStatus)
	err := c.cc.Invoke(ctx, "/messenger.NodeTide/GetClusterStatus", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *nodeTideClient) SendBlock(ctx context.Context, opts ...grpc.CallOption) (NodeTide_SendBlockClient, error) {
	stream, err := c.cc.NewStream(ctx, &NodeTide_ServiceDesc.Streams[0], "/messenger.NodeTide/SendBlock", opts...)
	if err != nil {
		return nil, err
	}
	x := &nodeTideSendBlockClient{stream}
	return x, nil
}

type NodeTide_SendBlockClient interface {
	Send(*messenger.Block) error
	CloseAndRecv() (*Result, error)
	grpc.ClientStream
}

type nodeTideSendBlockClient struct {
	grpc.ClientStream
}

func (x *nodeTideSendBlockClient) Send(m *messenger.Block) error {
	return x.ClientStream.SendMsg(m)
}

func (x *nodeTideSendBlockClient) CloseAndRecv() (*Result, error) {
	if err := x.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	m := new(Result)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

func (c *nodeTideClient) SendMetaData(ctx context.Context, in *messenger.ObjectInfo, opts ...grpc.CallOption) (*Result, error) {
	out := new(Result)
	err := c.cc.Invoke(ctx, "/messenger.NodeTide/SendMetaData", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// NodeTideServer is the server API for NodeTide service.
// All implementations must embed UnimplementedNodeTideServer
// for forward compatibility
type NodeTideServer interface {
	GetClusterStatus(context.Context, *emptypb.Empty) (*ClusterStatus, error)
	SendBlock(NodeTide_SendBlockServer) error
	SendMetaData(context.Context, *messenger.ObjectInfo) (*Result, error)
	mustEmbedUnimplementedNodeTideServer()
}

// UnimplementedNodeTideServer must be embedded to have forward compatible implementations.
type UnimplementedNodeTideServer struct {
}

func (UnimplementedNodeTideServer) GetClusterStatus(context.Context, *emptypb.Empty) (*ClusterStatus, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetClusterStatus not implemented")
}
func (UnimplementedNodeTideServer) SendBlock(NodeTide_SendBlockServer) error {
	return status.Errorf(codes.Unimplemented, "method SendBlock not implemented")
}
func (UnimplementedNodeTideServer) SendMetaData(context.Context, *messenger.ObjectInfo) (*Result, error) {
	return nil, status.Errorf(codes.Unimplemented, "method SendMetaData not implemented")
}
func (UnimplementedNodeTideServer) mustEmbedUnimplementedNodeTideServer() {}

// UnsafeNodeTideServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to NodeTideServer will
// result in compilation errors.
type UnsafeNodeTideServer interface {
	mustEmbedUnimplementedNodeTideServer()
}

func RegisterNodeTideServer(s grpc.ServiceRegistrar, srv NodeTideServer) {
	s.RegisterService(&NodeTide_ServiceDesc, srv)
}

func _NodeTide_GetClusterStatus_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(emptypb.Empty)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(NodeTideServer).GetClusterStatus(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/messenger.NodeTide/GetClusterStatus",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(NodeTideServer).GetClusterStatus(ctx, req.(*emptypb.Empty))
	}
	return interceptor(ctx, in, info, handler)
}

func _NodeTide_SendBlock_Handler(srv interface{}, stream grpc.ServerStream) error {
	return srv.(NodeTideServer).SendBlock(&nodeTideSendBlockServer{stream})
}

type NodeTide_SendBlockServer interface {
	SendAndClose(*Result) error
	Recv() (*messenger.Block, error)
	grpc.ServerStream
}

type nodeTideSendBlockServer struct {
	grpc.ServerStream
}

func (x *nodeTideSendBlockServer) SendAndClose(m *Result) error {
	return x.ServerStream.SendMsg(m)
}

func (x *nodeTideSendBlockServer) Recv() (*messenger.Block, error) {
	m := new(messenger.Block)
	if err := x.ServerStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

func _NodeTide_SendMetaData_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(messenger.ObjectInfo)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(NodeTideServer).SendMetaData(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/messenger.NodeTide/SendMetaData",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(NodeTideServer).SendMetaData(ctx, req.(*messenger.ObjectInfo))
	}
	return interceptor(ctx, in, info, handler)
}

// NodeTide_ServiceDesc is the grpc.ServiceDesc for NodeTide service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var NodeTide_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "messenger.NodeTide",
	HandlerType: (*NodeTideServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "GetClusterStatus",
			Handler:    _NodeTide_GetClusterStatus_Handler,
		},
		{
			MethodName: "SendMetaData",
			Handler:    _NodeTide_SendMetaData_Handler,
		},
	},
	Streams: []grpc.StreamDesc{
		{
			StreamName:    "SendBlock",
			Handler:       _NodeTide_SendBlock_Handler,
			ClientStreams: true,
		},
	},
	Metadata: "tide.proto",
}
