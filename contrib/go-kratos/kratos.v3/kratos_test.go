// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2026 Datadog, Inc.

package kratos

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/DataDog/dd-trace-go/v2/ddtrace/ext"
	"github.com/DataDog/dd-trace-go/v2/ddtrace/mocktracer"
	"github.com/DataDog/dd-trace-go/v2/ddtrace/tracer"
	"github.com/DataDog/dd-trace-go/v2/instrumentation"

	kratoserrors "github.com/go-kratos/kratos/v3/errors"
	"github.com/go-kratos/kratos/v3/middleware"
	"github.com/go-kratos/kratos/v3/transport"
	kratoshttp "github.com/go-kratos/kratos/v3/transport/http"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServerHTTP(t *testing.T) {
	mt := mocktracer.Start()
	defer mt.Stop()

	parent := tracer.StartSpan("upstream")
	header := testHeader{}
	require.NoError(t, tracer.Inject(parent.Context(), headerCarrier{Header: header}))
	parent.Finish()

	req, err := http.NewRequest(http.MethodPost, "http://example.com/v1/greeters/alice", nil)
	require.NoError(t, err)
	tr := &testTransport{
		kind:         transport.KindHTTP,
		operation:    "/helloworld.v1.Greeter/SayHello",
		header:       header,
		request:      req,
		pathTemplate: "/v1/greeters/{name}",
	}
	ctx := transport.NewServerContext(context.Background(), tr)

	var activeSpanID uint64
	next := func(ctx context.Context, req any) (any, error) {
		span, ok := tracer.SpanFromContext(ctx)
		require.True(t, ok)
		activeSpanID = span.Context().SpanID()
		return req, nil
	}
	reply, err := Server()(next)(ctx, "reply")
	require.NoError(t, err)
	assert.Equal(t, "reply", reply)

	span := findSpan(t, mt, "kratos.server.request")
	assert.Equal(t, activeSpanID, span.SpanID())
	assert.Equal(t, parent.Context().SpanID(), span.ParentID())
	assert.Equal(t, parent.Context().TraceID(), span.Context().TraceID())
	assert.Equal(t, "kratos", span.Tag(ext.ServiceName))
	assert.Equal(t, "/helloworld.v1.Greeter/SayHello", span.Tag(ext.ResourceName))
	assert.Equal(t, ext.SpanTypeWeb, span.Tag(ext.SpanType))
	assert.Equal(t, ext.SpanKindServer, span.Tag(ext.SpanKind))
	assert.Equal(t, "go-kratos/kratos.v3", span.Tag(ext.Component))
	assert.Equal(t, string(instrumentation.PackageGoKratosV3), span.Integration())
	assert.Equal(t, "http", span.Tag(ext.RPCSystem))
	assert.Equal(t, "helloworld.v1.Greeter", span.Tag(ext.RPCService))
	assert.Equal(t, "SayHello", span.Tag(ext.RPCMethod))
	assert.Equal(t, http.MethodPost, span.Tag(ext.HTTPMethod))
	assert.Equal(t, "/v1/greeters/alice", span.Tag(ext.HTTPURL))
	assert.Equal(t, "/v1/greeters/{name}", span.Tag(ext.HTTPRoute))
}

func TestClientGRPC(t *testing.T) {
	mt := mocktracer.Start()
	defer mt.Stop()

	parent, ctx := tracer.StartSpanFromContext(context.Background(), "parent")
	header := testHeader{}
	tr := &testTransport{
		kind:      transport.KindGRPC,
		operation: "/helloworld.v1.Greeter/SayHello",
		header:    header,
	}
	ctx = transport.NewClientContext(ctx, tr)

	next := func(ctx context.Context, req any) (any, error) {
		span, ok := tracer.SpanFromContext(ctx)
		require.True(t, ok)
		assert.Equal(t, strconv.FormatUint(span.Context().TraceIDLower(), 10), header.Get(tracer.DefaultTraceIDHeader))
		return req, nil
	}
	reply, err := Client()(next)(ctx, "reply")
	require.NoError(t, err)
	assert.Equal(t, "reply", reply)
	parent.Finish()

	span := findSpan(t, mt, "kratos.client.request")
	assert.Equal(t, parent.Context().SpanID(), span.ParentID())
	assert.Equal(t, ext.AppTypeRPC, span.Tag(ext.SpanType))
	assert.Equal(t, ext.SpanKindClient, span.Tag(ext.SpanKind))
	assert.Equal(t, ext.RPCSystemGRPC, span.Tag(ext.RPCSystem))
	assert.Equal(t, "/helloworld.v1.Greeter/SayHello", span.Tag(ext.GRPCFullMethod))
	assert.Equal(t, "helloworld.v1.Greeter", span.Tag(ext.RPCService))
	assert.Equal(t, "SayHello", span.Tag(ext.RPCMethod))
}

func TestHTTPTransportEndToEnd(t *testing.T) {
	mt := mocktracer.Start()
	defer mt.Stop()

	server := kratoshttp.NewServer(kratoshttp.Middleware(Server(WithService("kratos-server"))))
	server.Route("/").GET("/hello/{name}", func(ctx kratoshttp.Context) error {
		kratoshttp.SetOperation(ctx, "/helloworld.v1.Greeter/SayHello")
		handler := ctx.Middleware(func(ctx context.Context, _ any) (any, error) {
			_, ok := tracer.SpanFromContext(ctx)
			require.True(t, ok)
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

	client, err := kratoshttp.NewClient(
		context.Background(),
		kratoshttp.WithEndpoint(httpServer.URL),
		kratoshttp.WithMiddleware(Client(WithService("kratos-client"))),
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

	clientSpan := findSpan(t, mt, "kratos.client.request")
	serverSpan := findSpan(t, mt, "kratos.server.request")
	assert.Equal(t, clientSpan.TraceID(), serverSpan.TraceID())
	assert.Equal(t, clientSpan.SpanID(), serverSpan.ParentID())
	assert.Equal(t, "kratos-client", clientSpan.Tag(ext.ServiceName))
	assert.Equal(t, "kratos-server", serverSpan.Tag(ext.ServiceName))
}

func TestErrorAndOptions(t *testing.T) {
	mt := mocktracer.Start()
	defer mt.Stop()

	req, err := http.NewRequest(http.MethodGet, "http://example.com/v1/fail", nil)
	require.NoError(t, err)
	tr := &testTransport{
		kind:      transport.KindHTTP,
		operation: "/example.v1.Service/Fail",
		header:    testHeader{},
		request:   req,
	}
	ctx := transport.NewClientContext(context.Background(), tr)
	wantErr := kratoserrors.New(http.StatusServiceUnavailable, "DEPENDENCY_DOWN", "dependency unavailable")
	next := func(context.Context, any) (any, error) {
		return nil, wantErr
	}
	_, err = Client(
		WithService("kratos-client-test"),
		WithAnalyticsRate(0.5),
		WithSpanOptions(tracer.Tag("custom.tag", "custom-value")),
		NoDebugStack(),
	)(next)(ctx, nil)
	require.ErrorIs(t, err, wantErr)

	span := findSpan(t, mt, "kratos.client.request")
	assert.Equal(t, "kratos-client-test", span.Tag(ext.ServiceName))
	assert.Equal(t, float64(0.5), span.Tag(ext.EventSampleRate))
	assert.Equal(t, "custom-value", span.Tag("custom.tag"))
	assert.Equal(t, float64(http.StatusServiceUnavailable), span.Tag("kratos.status_code"))
	assert.Equal(t, "DEPENDENCY_DOWN", span.Tag("kratos.error_reason"))
	assert.Equal(t, "503", span.Tag(ext.HTTPCode))
	assert.Equal(t, wantErr.Error(), span.Tag(ext.ErrorMsg))
}

func TestWithoutTransportContext(t *testing.T) {
	mt := mocktracer.Start()
	defer mt.Stop()

	next := func(_ context.Context, req any) (any, error) {
		return req, nil
	}
	for name, mw := range map[string]middleware.Middleware{
		"server": Server(),
		"client": Client(),
	} {
		t.Run(name, func(t *testing.T) {
			reply, err := mw(next)(context.Background(), name)
			require.NoError(t, err)
			assert.Equal(t, name, reply)
		})
	}
	assert.Empty(t, mt.FinishedSpans())
}

func TestSplitOperation(t *testing.T) {
	for _, tc := range []struct {
		operation string
		service   string
		method    string
	}{
		{operation: "/helloworld.v1.Greeter/SayHello", service: "helloworld.v1.Greeter", method: "SayHello"},
		{operation: "health", service: "health"},
		{},
	} {
		t.Run(tc.operation, func(t *testing.T) {
			service, method := splitOperation(tc.operation)
			assert.Equal(t, tc.service, service)
			assert.Equal(t, tc.method, method)
		})
	}
}

func findSpan(t *testing.T, mt mocktracer.Tracer, operation string) *mocktracer.Span {
	t.Helper()
	for _, span := range mt.FinishedSpans() {
		if span.OperationName() == operation {
			return span
		}
	}
	require.FailNow(t, "span not found", "operation: %s", operation)
	return nil
}

type testHeader map[string][]string

func (h testHeader) Get(key string) string {
	values := h.Values(key)
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

func (h testHeader) Set(key, value string) {
	h[key] = []string{value}
}

func (h testHeader) Add(key, value string) {
	h[key] = append(h[key], value)
}

func (h testHeader) Keys() []string {
	keys := make([]string, 0, len(h))
	for key := range h {
		keys = append(keys, key)
	}
	return keys
}

func (h testHeader) Values(key string) []string {
	return h[key]
}

type testTransport struct {
	kind         transport.Kind
	endpoint     string
	operation    string
	header       testHeader
	request      *http.Request
	pathTemplate string
}

var _ kratoshttp.Transporter = (*testTransport)(nil)

func (tr *testTransport) Kind() transport.Kind            { return tr.kind }
func (tr *testTransport) Endpoint() string                { return tr.endpoint }
func (tr *testTransport) Operation() string               { return tr.operation }
func (tr *testTransport) RequestHeader() transport.Header { return tr.header }
func (tr *testTransport) ReplyHeader() transport.Header   { return nil }
func (tr *testTransport) Request() *http.Request          { return tr.request }
func (tr *testTransport) PathTemplate() string            { return tr.pathTemplate }
