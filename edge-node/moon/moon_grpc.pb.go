// Code generated by protoc-gen-go-grpc. DO NOT EDIT.

package moon

import (
	context "context"
	node "ecos/edge-node/node"
	raftpb "go.etcd.io/etcd/raft/v3/raftpb"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
)

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
// Requires gRPC-Go v1.32.0 or later.
const _ = grpc.SupportPackageIsVersion7

// MoonClient is the client API for Moon service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type MoonClient interface {
	SendRaftMessage(ctx context.Context, in *raftpb.Message, opts ...grpc.CallOption) (*raftpb.Message, error)
	AddNodeToGroup(ctx context.Context, in *node.NodeInfo, opts ...grpc.CallOption) (*AddNodeReply, error)
	GetGroupInfo(ctx context.Context, in *GetGroupInfoRequest, opts ...grpc.CallOption) (*node.GroupInfo, error)
}

type moonClient struct {
	cc grpc.ClientConnInterface
}

func NewMoonClient(cc grpc.ClientConnInterface) MoonClient {
	return &moonClient{cc}
}

func (c *moonClient) SendRaftMessage(ctx context.Context, in *raftpb.Message, opts ...grpc.CallOption) (*raftpb.Message, error) {
	out := new(raftpb.Message)
	err := c.cc.Invoke(ctx, "/messenger.Moon/SendRaftMessage", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *moonClient) AddNodeToGroup(ctx context.Context, in *node.NodeInfo, opts ...grpc.CallOption) (*AddNodeReply, error) {
	out := new(AddNodeReply)
	err := c.cc.Invoke(ctx, "/messenger.Moon/AddNodeToGroup", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *moonClient) GetGroupInfo(ctx context.Context, in *GetGroupInfoRequest, opts ...grpc.CallOption) (*node.GroupInfo, error) {
	out := new(node.GroupInfo)
	err := c.cc.Invoke(ctx, "/messenger.Moon/GetGroupInfo", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// MoonServer is the server API for Moon service.
// All implementations must embed UnimplementedMoonServer
// for forward compatibility
type MoonServer interface {
	SendRaftMessage(context.Context, *raftpb.Message) (*raftpb.Message, error)
	AddNodeToGroup(context.Context, *node.NodeInfo) (*AddNodeReply, error)
	GetGroupInfo(context.Context, *GetGroupInfoRequest) (*node.GroupInfo, error)
	mustEmbedUnimplementedMoonServer()
}

// UnimplementedMoonServer must be embedded to have forward compatible implementations.
type UnimplementedMoonServer struct {
}

func (UnimplementedMoonServer) SendRaftMessage(context.Context, *raftpb.Message) (*raftpb.Message, error) {
	return nil, status.Errorf(codes.Unimplemented, "method SendRaftMessage not implemented")
}
func (UnimplementedMoonServer) AddNodeToGroup(context.Context, *node.NodeInfo) (*AddNodeReply, error) {
	return nil, status.Errorf(codes.Unimplemented, "method AddNodeToGroup not implemented")
}
func (UnimplementedMoonServer) GetGroupInfo(context.Context, *GetGroupInfoRequest) (*node.GroupInfo, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetGroupInfo not implemented")
}
func (UnimplementedMoonServer) mustEmbedUnimplementedMoonServer() {}

// UnsafeMoonServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to MoonServer will
// result in compilation errors.
type UnsafeMoonServer interface {
	mustEmbedUnimplementedMoonServer()
}

func RegisterMoonServer(s grpc.ServiceRegistrar, srv MoonServer) {
	s.RegisterService(&Moon_ServiceDesc, srv)
}

func _Moon_SendRaftMessage_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(raftpb.Message)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(MoonServer).SendRaftMessage(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/messenger.Moon/SendRaftMessage",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(MoonServer).SendRaftMessage(ctx, req.(*raftpb.Message))
	}
	return interceptor(ctx, in, info, handler)
}

func _Moon_AddNodeToGroup_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(node.NodeInfo)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(MoonServer).AddNodeToGroup(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/messenger.Moon/AddNodeToGroup",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(MoonServer).AddNodeToGroup(ctx, req.(*node.NodeInfo))
	}
	return interceptor(ctx, in, info, handler)
}

func _Moon_GetGroupInfo_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(GetGroupInfoRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(MoonServer).GetGroupInfo(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/messenger.Moon/GetGroupInfo",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(MoonServer).GetGroupInfo(ctx, req.(*GetGroupInfoRequest))
	}
	return interceptor(ctx, in, info, handler)
}

// Moon_ServiceDesc is the grpc.ServiceDesc for Moon service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var Moon_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "messenger.Moon",
	HandlerType: (*MoonServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "SendRaftMessage",
			Handler:    _Moon_SendRaftMessage_Handler,
		},
		{
			MethodName: "AddNodeToGroup",
			Handler:    _Moon_AddNodeToGroup_Handler,
		},
		{
			MethodName: "GetGroupInfo",
			Handler:    _Moon_GetGroupInfo_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "moon.proto",
}
