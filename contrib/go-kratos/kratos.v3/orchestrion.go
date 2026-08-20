// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2026 Datadog, Inc.

package kratos

import (
	"context"
	"reflect"
	"unsafe"

	"github.com/go-kratos/kratos/v3/middleware"
	kratosgrpc "github.com/go-kratos/kratos/v3/transport/grpc"
	kratoshttp "github.com/go-kratos/kratos/v3/transport/http"
	"google.golang.org/grpc"
)

// OrchestrionNewHTTPServer is used by Orchestrion to add Kratos HTTP server tracing.
func OrchestrionNewHTTPServer(opts ...kratoshttp.ServerOption) *kratoshttp.Server {
	opts = append(opts, kratoshttp.Middleware(Server()))
	return kratoshttp.NewServer(opts...)
}

// OrchestrionNewHTTPClient is used by Orchestrion to add Kratos HTTP client tracing.
func OrchestrionNewHTTPClient(ctx context.Context, opts ...kratoshttp.ClientOption) (*kratoshttp.Client, error) {
	opts = appendHTTPClientTracing(opts)
	return kratoshttp.NewClient(ctx, opts...)
}

// OrchestrionNewGRPCServer is used by Orchestrion to add Kratos gRPC server tracing.
func OrchestrionNewGRPCServer(opts ...kratosgrpc.ServerOption) *kratosgrpc.Server {
	opts = append(opts, kratosgrpc.Middleware(Server()))
	return kratosgrpc.NewServer(opts...)
}

// OrchestrionNewGRPCClient is used by Orchestrion to add Kratos gRPC client tracing.
func OrchestrionNewGRPCClient(ctx context.Context, opts ...kratosgrpc.ClientOption) (*grpc.ClientConn, error) {
	opts = appendGRPCClientTracing(opts)
	return kratosgrpc.NewClient(ctx, opts...)
}

func appendHTTPClientTracing(opts []kratoshttp.ClientOption) []kratoshttp.ClientOption {
	return append(opts, clientMiddlewareOption[kratoshttp.ClientOption](
		reflect.TypeOf(kratoshttp.WithMiddleware()),
		Client(),
	))
}

func appendGRPCClientTracing(opts []kratosgrpc.ClientOption) []kratosgrpc.ClientOption {
	return append(opts, clientMiddlewareOption[kratosgrpc.ClientOption](
		reflect.TypeOf(kratosgrpc.WithMiddleware()),
		Client(),
	))
}

// clientMiddlewareOption creates a Kratos ClientOption which appends to the
// middleware configured by earlier options. Kratos exposes ClientOption but
// keeps its argument type private, so reflection is required to preserve user
// middleware instead of replacing it with tracing middleware.
func clientMiddlewareOption[T any](optionType reflect.Type, tracing middleware.Middleware) T {
	option := reflect.MakeFunc(optionType, func(args []reflect.Value) []reflect.Value {
		field := args[0].Elem().FieldByName("middleware")
		if !field.IsValid() {
			panic("kratos client options no longer contain middleware")
		}

		configured := (*[]middleware.Middleware)(unsafe.Pointer(field.UnsafeAddr()))
		*configured = append(*configured, tracing)
		return nil
	})
	return option.Interface().(T)
}
