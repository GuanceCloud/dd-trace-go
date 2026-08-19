// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2026 Datadog, Inc.

package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"

	kratostrace "github.com/DataDog/dd-trace-go/contrib/go-kratos/kratos.v3/v2"
	"github.com/DataDog/dd-trace-go/v2/ddtrace/tracer"
	kratoshttp "github.com/go-kratos/kratos/v3/transport/http"
)

const operation = "/helloworld.v1.Greeter/SayHello"

func main() {
	tracer.Start(
		tracer.WithAgentAddr("127.0.0.1:9529"),
		tracer.WithService("kratos-v3-demo"),
		tracer.WithEnv("local"),
		tracer.WithDebugMode(true),
	)
	defer tracer.Stop()

	server := kratoshttp.NewServer(
		kratoshttp.Middleware(kratostrace.Server(kratostrace.WithService("kratos-v3-demo-server"))),
	)
	server.Route("/").GET("/hello/{name}", func(ctx kratoshttp.Context) error {
		kratoshttp.SetOperation(ctx, operation)
		handler := ctx.Middleware(func(context.Context, any) (any, error) {
			return map[string]string{"message": "hello kratos v3"}, nil
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
		kratoshttp.WithMiddleware(kratostrace.Client(kratostrace.WithService("kratos-v3-demo-client"))),
	)
	if err != nil {
		panic(err)
	}
	defer client.Close()

	var reply map[string]string
	if err := client.Invoke(
		context.Background(),
		http.MethodGet,
		"/hello/datadog",
		nil,
		&reply,
		kratoshttp.Operation(operation),
		kratoshttp.PathTemplate("/hello/{name}"),
	); err != nil {
		panic(err)
	}
	fmt.Println(reply["message"])
}
