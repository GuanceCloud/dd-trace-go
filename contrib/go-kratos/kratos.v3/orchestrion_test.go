// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2026 Datadog, Inc.

package kratos

import (
	"context"
	"reflect"
	"testing"

	kratosgrpc "github.com/go-kratos/kratos/v3/transport/grpc"
	kratoshttp "github.com/go-kratos/kratos/v3/transport/http"
	"github.com/stretchr/testify/require"
)

func TestOrchestrionHTTPClientAppliesOptionsOnce(t *testing.T) {
	var calls int
	optionType := reflect.TypeOf(kratoshttp.WithMiddleware())
	sideEffect := reflect.MakeFunc(optionType, func([]reflect.Value) []reflect.Value {
		calls++
		return nil
	}).Interface().(kratoshttp.ClientOption)

	client, err := OrchestrionNewHTTPClient(
		context.Background(),
		kratoshttp.WithEndpoint("http://127.0.0.1:1"),
		sideEffect,
	)
	require.NoError(t, err)
	require.NoError(t, client.Close())
	require.Equal(t, 1, calls)
}

func TestOrchestrionGRPCClientAppliesOptionsOnce(t *testing.T) {
	var calls int
	optionType := reflect.TypeOf(kratosgrpc.WithMiddleware())
	sideEffect := reflect.MakeFunc(optionType, func([]reflect.Value) []reflect.Value {
		calls++
		return nil
	}).Interface().(kratosgrpc.ClientOption)

	client, err := OrchestrionNewGRPCClient(
		context.Background(),
		kratosgrpc.WithEndpoint("127.0.0.1:1"),
		sideEffect,
	)
	require.NoError(t, err)
	require.NoError(t, client.Close())
	require.Equal(t, 1, calls)
}
