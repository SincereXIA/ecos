// Code generated by protoc-gen-go-grpc. DO NOT EDIT.

package watcher

import (
	context "context"
	common "ecos/messenger/common"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
	emptypb "google.golang.org/protobuf/types/known/emptypb"
)

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
// Requires gRPC-Go v1.32.0 or later.
const _ = grpc.SupportPackageIsVersion7

// MonitorClient is the client API for Monitor service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type MonitorClient interface {
	Report(ctx context.Context, in *NodeStatusReport, opts ...grpc.CallOption) (*common.Result, error)
	Get(ctx context.Context, in *emptypb.Empty, opts ...grpc.CallOption) (*NodeStatusReport, error)
	// GetClusterReport returns the entire cluster status report.
	// This only works if the node is the leader of the cluster.
	GetClusterReport(ctx context.Context, in *emptypb.Empty, opts ...grpc.CallOption) (*ClusterReport, error)
}

type monitorClient struct {
	cc grpc.ClientConnInterface
}

func NewMonitorClient(cc grpc.ClientConnInterface) MonitorClient {
	return &monitorClient{cc}
}

func (c *monitorClient) Report(ctx context.Context, in *NodeStatusReport, opts ...grpc.CallOption) (*common.Result, error) {
	out := new(common.Result)
	err := c.cc.Invoke(ctx, "/messenger.Monitor/Report", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *monitorClient) Get(ctx context.Context, in *emptypb.Empty, opts ...grpc.CallOption) (*NodeStatusReport, error) {
	out := new(NodeStatusReport)
	err := c.cc.Invoke(ctx, "/messenger.Monitor/Get", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *monitorClient) GetClusterReport(ctx context.Context, in *emptypb.Empty, opts ...grpc.CallOption) (*ClusterReport, error) {
	out := new(ClusterReport)
	err := c.cc.Invoke(ctx, "/messenger.Monitor/GetClusterReport", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// MonitorServer is the server API for Monitor service.
// All implementations must embed UnimplementedMonitorServer
// for forward compatibility
type MonitorServer interface {
	Report(context.Context, *NodeStatusReport) (*common.Result, error)
	Get(context.Context, *emptypb.Empty) (*NodeStatusReport, error)
	// GetClusterReport returns the entire cluster status report.
	// This only works if the node is the leader of the cluster.
	GetClusterReport(context.Context, *emptypb.Empty) (*ClusterReport, error)
	mustEmbedUnimplementedMonitorServer()
}

// UnimplementedMonitorServer must be embedded to have forward compatible implementations.
type UnimplementedMonitorServer struct {
}

func (UnimplementedMonitorServer) Report(context.Context, *NodeStatusReport) (*common.Result, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Report not implemented")
}
func (UnimplementedMonitorServer) Get(context.Context, *emptypb.Empty) (*NodeStatusReport, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Get not implemented")
}
func (UnimplementedMonitorServer) GetClusterReport(context.Context, *emptypb.Empty) (*ClusterReport, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetClusterReport not implemented")
}
func (UnimplementedMonitorServer) mustEmbedUnimplementedMonitorServer() {}

// UnsafeMonitorServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to MonitorServer will
// result in compilation errors.
type UnsafeMonitorServer interface {
	mustEmbedUnimplementedMonitorServer()
}

func RegisterMonitorServer(s grpc.ServiceRegistrar, srv MonitorServer) {
	s.RegisterService(&Monitor_ServiceDesc, srv)
}

func _Monitor_Report_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(NodeStatusReport)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(MonitorServer).Report(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/messenger.Monitor/Report",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(MonitorServer).Report(ctx, req.(*NodeStatusReport))
	}
	return interceptor(ctx, in, info, handler)
}

func _Monitor_Get_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(emptypb.Empty)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(MonitorServer).Get(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/messenger.Monitor/Get",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(MonitorServer).Get(ctx, req.(*emptypb.Empty))
	}
	return interceptor(ctx, in, info, handler)
}

func _Monitor_GetClusterReport_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(emptypb.Empty)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(MonitorServer).GetClusterReport(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/messenger.Monitor/GetClusterReport",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(MonitorServer).GetClusterReport(ctx, req.(*emptypb.Empty))
	}
	return interceptor(ctx, in, info, handler)
}

// Monitor_ServiceDesc is the grpc.ServiceDesc for Monitor service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var Monitor_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "messenger.Monitor",
	HandlerType: (*MonitorServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "Report",
			Handler:    _Monitor_Report_Handler,
		},
		{
			MethodName: "Get",
			Handler:    _Monitor_Get_Handler,
		},
		{
			MethodName: "GetClusterReport",
			Handler:    _Monitor_GetClusterReport_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "monitor.proto",
}
