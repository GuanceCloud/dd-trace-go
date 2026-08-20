// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2026 Datadog, Inc.

package kratosv3

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/health/grpc_health_v1"

	"github.com/DataDog/dd-trace-go/v2/ddtrace/ext"
	"github.com/DataDog/dd-trace-go/v2/ddtrace/tracer"
	"github.com/DataDog/dd-trace-go/v2/internal/orchestrion/_integration/internal/trace"
	"github.com/go-kratos/kratos/v3/middleware"
	kratosgrpc "github.com/go-kratos/kratos/v3/transport/grpc"
	kratoshttp "github.com/go-kratos/kratos/v3/transport/http"
)

type TestCaseHTTP struct{}

func (*TestCaseHTTP) Setup(context.Context, *testing.T) {}

func (*TestCaseHTTP) Run(_ context.Context, t *testing.T) {
	server := kratoshttp.NewServer()
	server.Route("/").GET("/hello/{name}", func(ctx kratoshttp.Context) error {
		kratoshttp.SetOperation(ctx, "/helloworld.v1.Greeter/SayHello")
		handler := ctx.Middleware(func(ctx context.Context, _ any) (any, error) {
			if _, ok := tracer.SpanFromContext(ctx); !ok {
				return nil, errors.New("active span missing from handler context")
			}
			return map[string]string{"message": "hello"}, nil
		})
		reply, err := handler(ctx, nil)
		if err != nil {
			return err
		}
		return ctx.JSON(http.StatusOK, reply)
	})
	httpServer := httptest.NewServer(server)
	defer httpServer.Close()

	var userMiddlewareCalled bool
	userMiddleware := func(next middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req any) (any, error) {
			userMiddlewareCalled = true
			return next(ctx, req)
		}
	}
	client, err := kratoshttp.NewClient(
		context.Background(),
		kratoshttp.WithEndpoint(httpServer.URL),
		kratoshttp.WithMiddleware(userMiddleware),
	)
	require.NoError(t, err)
	defer client.Close()

	var reply map[string]string
	err = client.Invoke(
		context.Background(),
		http.MethodGet,
		"/hello/alice",
		nil,
		&reply,
		kratoshttp.Operation("/helloworld.v1.Greeter/SayHello"),
		kratoshttp.PathTemplate("/hello/{name}"),
	)
	require.NoError(t, err)
	assert.Equal(t, "hello", reply["message"])
	assert.True(t, userMiddlewareCalled, "automatic tracing must preserve HTTP client middleware")
}

func (*TestCaseHTTP) ExpectedTraces() trace.Traces {
	return trace.Traces{
		{
			Tags: map[string]any{
				"name":       "http.request",
				"resource":   "/helloworld.v1.Greeter/SayHello",
				"type":       "http",
				"component":  "go-kratos/kratos.v3",
				"span.kind":  ext.SpanKindClient,
				"rpc.system": "http",
			},
		},
		{
			Tags: map[string]any{
				"name":       "http.request",
				"resource":   "/helloworld.v1.Greeter/SayHello",
				"type":       "web",
				"component":  "go-kratos/kratos.v3",
				"span.kind":  ext.SpanKindServer,
				"rpc.system": "http",
			},
		},
	}
}

type TestCaseGRPC struct{}

func (*TestCaseGRPC) Setup(context.Context, *testing.T) {}

func (*TestCaseGRPC) Run(_ context.Context, t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	server := kratosgrpc.NewServer(
		kratosgrpc.Listener(listener),
	)
	serverDone := make(chan error, 1)
	go func() {
		serverDone <- server.Start(context.Background())
	}()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		require.NoError(t, server.Stop(ctx))
		require.NoError(t, <-serverDone)
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var userMiddlewareCalled bool
	userMiddleware := func(next middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req any) (any, error) {
			userMiddlewareCalled = true
			return next(ctx, req)
		}
	}
	conn, err := kratosgrpc.NewClient(
		ctx,
		kratosgrpc.WithEndpoint(listener.Addr().String()),
		kratosgrpc.WithMiddleware(userMiddleware),
	)
	require.NoError(t, err)
	defer conn.Close()

	_, err = grpc_health_v1.NewHealthClient(conn).Check(ctx, &grpc_health_v1.HealthCheckRequest{})
	require.NoError(t, err)
	assert.True(t, userMiddlewareCalled, "automatic tracing must preserve gRPC client middleware")
}

func (*TestCaseGRPC) ExpectedTraces() trace.Traces {
	return trace.Traces{
		{
			Tags: map[string]any{
				"name":        "grpc.client",
				"resource":    "/grpc.health.v1.Health/Check",
				"type":        "rpc",
				"component":   "go-kratos/kratos.v3",
				"span.kind":   ext.SpanKindClient,
				"rpc.system":  "grpc",
				"rpc.service": "grpc.health.v1.Health",
				"rpc.method":  "Check",
			},
		},
		{
			Tags: map[string]any{
				"name":        "grpc.server",
				"resource":    "/grpc.health.v1.Health/Check",
				"type":        "rpc",
				"component":   "go-kratos/kratos.v3",
				"span.kind":   ext.SpanKindServer,
				"rpc.system":  "grpc",
				"rpc.service": "grpc.health.v1.Health",
				"rpc.method":  "Check",
			},
		},
	}
}
