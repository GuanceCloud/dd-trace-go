// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2026 Datadog, Inc.

package kratos

import (
	"math"

	"github.com/DataDog/dd-trace-go/v2/ddtrace/tracer"
	"github.com/DataDog/dd-trace-go/v2/instrumentation"
)

type config struct {
	serviceName   string
	serviceSource string
	spanName      string
	analyticsRate float64
	noDebugStack  bool
	spanOpts      []tracer.StartSpanOption
}

// Option configures the Kratos tracing middleware.
type Option func(*config)

func applyOptions(cfg *config, opts []Option) {
	for _, opt := range opts {
		opt(cfg)
	}
}

func defaults(cfg *config) {
	cfg.analyticsRate = instr.AnalyticsRate(false)
}

func serverDefaults(cfg *config) {
	cfg.serviceName = instr.ServiceName(instrumentation.ComponentServer, nil)
	cfg.serviceSource = string(component)
	cfg.spanName = instr.OperationName(instrumentation.ComponentServer, nil)
	defaults(cfg)
}

func clientDefaults(cfg *config) {
	cfg.serviceName = instr.ServiceName(instrumentation.ComponentClient, nil)
	cfg.serviceSource = string(component)
	cfg.spanName = instr.OperationName(instrumentation.ComponentClient, nil)
	defaults(cfg)
}

// WithService sets the service name for spans created by the middleware.
func WithService(name string) Option {
	return func(cfg *config) {
		cfg.serviceName = name
		cfg.serviceSource = instrumentation.ServiceSourceWithServiceOption
	}
}

// WithAnalytics enables or disables Trace Analytics for created spans.
func WithAnalytics(enabled bool) Option {
	if enabled {
		return WithAnalyticsRate(1)
	}
	return WithAnalyticsRate(math.NaN())
}

// WithAnalyticsRate sets the Trace Analytics sampling rate for created spans.
func WithAnalyticsRate(rate float64) Option {
	return func(cfg *config) {
		if rate >= 0 && rate <= 1 {
			cfg.analyticsRate = rate
		} else {
			cfg.analyticsRate = math.NaN()
		}
	}
}

// NoDebugStack prevents error spans from collecting a Go stack trace.
func NoDebugStack() Option {
	return func(cfg *config) {
		cfg.noDebugStack = true
	}
}

// WithSpanOptions adds options to every span created by the middleware.
func WithSpanOptions(opts ...tracer.StartSpanOption) Option {
	return func(cfg *config) {
		cfg.spanOpts = append(cfg.spanOpts, opts...)
	}
}
