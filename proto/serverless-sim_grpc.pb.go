// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.3.0
// - protoc             v4.24.3
// source: serverless-sim.proto

package schedulerproto

import (
	context "context"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
)

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
// Requires gRPC-Go v1.32.0 or later.
const _ = grpc.SupportPackageIsVersion7

const (
	Scaler_Assign_FullMethodName = "/serverless.simulator.Scaler/Assign"
	Scaler_Idle_FullMethodName   = "/serverless.simulator.Scaler/Idle"
)

// ScalerClient is the client API for Scaler service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type ScalerClient interface {
	Assign(ctx context.Context, in *AssignRequest, opts ...grpc.CallOption) (*AssignReply, error)
	Idle(ctx context.Context, in *IdleRequest, opts ...grpc.CallOption) (*IdleReply, error)
}

type scalerClient struct {
	cc grpc.ClientConnInterface
}

func NewScalerClient(cc grpc.ClientConnInterface) ScalerClient {
	return &scalerClient{cc}
}

func (c *scalerClient) Assign(ctx context.Context, in *AssignRequest, opts ...grpc.CallOption) (*AssignReply, error) {
	out := new(AssignReply)
	err := c.cc.Invoke(ctx, Scaler_Assign_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *scalerClient) Idle(ctx context.Context, in *IdleRequest, opts ...grpc.CallOption) (*IdleReply, error) {
	out := new(IdleReply)
	err := c.cc.Invoke(ctx, Scaler_Idle_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// ScalerServer is the server API for Scaler service.
// All implementations must embed UnimplementedScalerServer
// for forward compatibility
type ScalerServer interface {
	Assign(context.Context, *AssignRequest) (*AssignReply, error)
	Idle(context.Context, *IdleRequest) (*IdleReply, error)
	mustEmbedUnimplementedScalerServer()
}

// UnimplementedScalerServer must be embedded to have forward compatible implementations.
type UnimplementedScalerServer struct {
}

func (UnimplementedScalerServer) Assign(context.Context, *AssignRequest) (*AssignReply, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Assign not implemented")
}
func (UnimplementedScalerServer) Idle(context.Context, *IdleRequest) (*IdleReply, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Idle not implemented")
}
func (UnimplementedScalerServer) mustEmbedUnimplementedScalerServer() {}

// UnsafeScalerServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to ScalerServer will
// result in compilation errors.
type UnsafeScalerServer interface {
	mustEmbedUnimplementedScalerServer()
}

func RegisterScalerServer(s grpc.ServiceRegistrar, srv ScalerServer) {
	s.RegisterService(&Scaler_ServiceDesc, srv)
}

func _Scaler_Assign_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(AssignRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ScalerServer).Assign(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Scaler_Assign_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ScalerServer).Assign(ctx, req.(*AssignRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Scaler_Idle_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(IdleRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ScalerServer).Idle(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Scaler_Idle_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ScalerServer).Idle(ctx, req.(*IdleRequest))
	}
	return interceptor(ctx, in, info, handler)
}

// Scaler_ServiceDesc is the grpc.ServiceDesc for Scaler service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var Scaler_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "serverless.simulator.Scaler",
	HandlerType: (*ScalerServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "Assign",
			Handler:    _Scaler_Assign_Handler,
		},
		{
			MethodName: "Idle",
			Handler:    _Scaler_Idle_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "serverless-sim.proto",
}

const (
	Platform_CreateSlot_FullMethodName  = "/serverless.simulator.Platform/CreateSlot"
	Platform_DestroySlot_FullMethodName = "/serverless.simulator.Platform/DestroySlot"
	Platform_Init_FullMethodName        = "/serverless.simulator.Platform/Init"
)

// PlatformClient is the client API for Platform service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type PlatformClient interface {
	// Slot
	CreateSlot(ctx context.Context, in *CreateSlotRequest, opts ...grpc.CallOption) (*CreateSlotReply, error)
	DestroySlot(ctx context.Context, in *DestroySlotRequest, opts ...grpc.CallOption) (*DestroySlotReply, error)
	// Init
	Init(ctx context.Context, in *InitRequest, opts ...grpc.CallOption) (*InitReply, error)
}

type platformClient struct {
	cc grpc.ClientConnInterface
}

func NewPlatformClient(cc grpc.ClientConnInterface) PlatformClient {
	return &platformClient{cc}
}

func (c *platformClient) CreateSlot(ctx context.Context, in *CreateSlotRequest, opts ...grpc.CallOption) (*CreateSlotReply, error) {
	out := new(CreateSlotReply)
	err := c.cc.Invoke(ctx, Platform_CreateSlot_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *platformClient) DestroySlot(ctx context.Context, in *DestroySlotRequest, opts ...grpc.CallOption) (*DestroySlotReply, error) {
	out := new(DestroySlotReply)
	err := c.cc.Invoke(ctx, Platform_DestroySlot_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *platformClient) Init(ctx context.Context, in *InitRequest, opts ...grpc.CallOption) (*InitReply, error) {
	out := new(InitReply)
	err := c.cc.Invoke(ctx, Platform_Init_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// PlatformServer is the server API for Platform service.
// All implementations must embed UnimplementedPlatformServer
// for forward compatibility
type PlatformServer interface {
	// Slot
	CreateSlot(context.Context, *CreateSlotRequest) (*CreateSlotReply, error)
	DestroySlot(context.Context, *DestroySlotRequest) (*DestroySlotReply, error)
	// Init
	Init(context.Context, *InitRequest) (*InitReply, error)
	mustEmbedUnimplementedPlatformServer()
}

// UnimplementedPlatformServer must be embedded to have forward compatible implementations.
type UnimplementedPlatformServer struct {
}

func (UnimplementedPlatformServer) CreateSlot(context.Context, *CreateSlotRequest) (*CreateSlotReply, error) {
	return nil, status.Errorf(codes.Unimplemented, "method CreateSlot not implemented")
}
func (UnimplementedPlatformServer) DestroySlot(context.Context, *DestroySlotRequest) (*DestroySlotReply, error) {
	return nil, status.Errorf(codes.Unimplemented, "method DestroySlot not implemented")
}
func (UnimplementedPlatformServer) Init(context.Context, *InitRequest) (*InitReply, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Init not implemented")
}
func (UnimplementedPlatformServer) mustEmbedUnimplementedPlatformServer() {}

// UnsafePlatformServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to PlatformServer will
// result in compilation errors.
type UnsafePlatformServer interface {
	mustEmbedUnimplementedPlatformServer()
}

func RegisterPlatformServer(s grpc.ServiceRegistrar, srv PlatformServer) {
	s.RegisterService(&Platform_ServiceDesc, srv)
}

func _Platform_CreateSlot_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(CreateSlotRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(PlatformServer).CreateSlot(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Platform_CreateSlot_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(PlatformServer).CreateSlot(ctx, req.(*CreateSlotRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Platform_DestroySlot_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(DestroySlotRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(PlatformServer).DestroySlot(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Platform_DestroySlot_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(PlatformServer).DestroySlot(ctx, req.(*DestroySlotRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Platform_Init_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(InitRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(PlatformServer).Init(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Platform_Init_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(PlatformServer).Init(ctx, req.(*InitRequest))
	}
	return interceptor(ctx, in, info, handler)
}

// Platform_ServiceDesc is the grpc.ServiceDesc for Platform service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var Platform_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "serverless.simulator.Platform",
	HandlerType: (*PlatformServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "CreateSlot",
			Handler:    _Platform_CreateSlot_Handler,
		},
		{
			MethodName: "DestroySlot",
			Handler:    _Platform_DestroySlot_Handler,
		},
		{
			MethodName: "Init",
			Handler:    _Platform_Init_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "serverless-sim.proto",
}
