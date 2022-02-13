// Code generated by protoc-gen-go-grpc. DO NOT EDIT.

package alaya

import (
	context "context"
	common "ecos/messenger/common"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
)

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
// Requires gRPC-Go v1.32.0 or later.
const _ = grpc.SupportPackageIsVersion7

// AlayaClient is the client API for Alaya service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type AlayaClient interface {
	RecordObjectMeta(ctx context.Context, in *ObjectMeta, opts ...grpc.CallOption) (*common.Result, error)
}

type alayaClient struct {
	cc grpc.ClientConnInterface
}

func NewAlayaClient(cc grpc.ClientConnInterface) AlayaClient {
	return &alayaClient{cc}
}

func (c *alayaClient) RecordObjectMeta(ctx context.Context, in *ObjectMeta, opts ...grpc.CallOption) (*common.Result, error) {
	out := new(common.Result)
	err := c.cc.Invoke(ctx, "/messenger.Alaya/RecordObjectMeta", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// AlayaServer is the server API for Alaya service.
// All implementations must embed UnimplementedAlayaServer
// for forward compatibility
type AlayaServer interface {
	RecordObjectMeta(context.Context, *ObjectMeta) (*common.Result, error)
	mustEmbedUnimplementedAlayaServer()
}

// UnimplementedAlayaServer must be embedded to have forward compatible implementations.
type UnimplementedAlayaServer struct {
}

func (UnimplementedAlayaServer) RecordObjectMeta(context.Context, *ObjectMeta) (*common.Result, error) {
	return nil, status.Errorf(codes.Unimplemented, "method RecordObjectMeta not implemented")
}
func (UnimplementedAlayaServer) mustEmbedUnimplementedAlayaServer() {}

// UnsafeAlayaServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to AlayaServer will
// result in compilation errors.
type UnsafeAlayaServer interface {
	mustEmbedUnimplementedAlayaServer()
}

func RegisterAlayaServer(s grpc.ServiceRegistrar, srv AlayaServer) {
	s.RegisterService(&Alaya_ServiceDesc, srv)
}

func _Alaya_RecordObjectMeta_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ObjectMeta)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(AlayaServer).RecordObjectMeta(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/messenger.Alaya/RecordObjectMeta",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(AlayaServer).RecordObjectMeta(ctx, req.(*ObjectMeta))
	}
	return interceptor(ctx, in, info, handler)
}

// Alaya_ServiceDesc is the grpc.ServiceDesc for Alaya service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var Alaya_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "messenger.Alaya",
	HandlerType: (*AlayaServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "RecordObjectMeta",
			Handler:    _Alaya_RecordObjectMeta_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "alaya.proto",
}