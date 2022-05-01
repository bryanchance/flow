// Copyright 2022 Evan Hazlett
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package tracing

import (
	"context"
	"os"

	"github.com/fynca/fynca/version"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	tracesdk "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.7.0"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc/credentials"
)

func getHostname() string {
	hostname, err := os.Hostname()
	if err != nil {
		return "unknown"
	}

	return hostname
}

func newExporter(ctx context.Context, endpoint string, tlsInsecure bool) (*otlptrace.Exporter, error) {
	opts := []otlptracegrpc.Option{
		otlptracegrpc.WithEndpoint(endpoint),
		otlptracegrpc.WithHeaders(map[string]string{}),
	}
	if tlsInsecure {
		opts = append(opts, otlptracegrpc.WithInsecure())
	} else {
		opts = append(opts, otlptracegrpc.WithTLSCredentials(credentials.NewClientTLSFromCert(nil, "")))
	}

	client := otlptracegrpc.NewClient(opts...)
	return otlptrace.New(ctx, client)
}

func tracerProvider(endpoint, service, environment string) (*tracesdk.TracerProvider, error) {
	opts := []tracesdk.TracerProviderOption{
		tracesdk.WithSampler(tracesdk.AlwaysSample()),
		tracesdk.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceNameKey.String(service),
			attribute.String("environment", environment),
			attribute.String("host", getHostname()),
			attribute.String("version", version.FullVersion()),
		)),
	}
	if endpoint != "" {
		// TODO: support tls
		exporter, err := newExporter(context.Background(), endpoint, true)
		if err != nil {
			return nil, err
		}
		opts = append(opts, tracesdk.WithBatcher(exporter))
	}
	tp := tracesdk.NewTracerProvider(opts...)
	return tp, nil
}

// NewProvider configures and OpenTelemetry tracing provider
func NewProvider(endpoint string, serviceName, environment string) (*tracesdk.TracerProvider, error) {
	tp, err := tracerProvider(endpoint, serviceName, environment)
	if err != nil {
		return nil, err
	}

	// register global provider
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))

	return tp, nil
}

// StartSpan starts child span in a context.
func StartSpan(ctx context.Context, opName string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	if parent := trace.SpanFromContext(ctx); parent != nil && parent.SpanContext().IsValid() {
		return parent.TracerProvider().Tracer("").Start(ctx, opName, opts...)
	}
	return otel.Tracer("fynca").Start(ctx, opName, opts...)
}

// StopSpan ends the span specified
func StopSpan(span trace.Span) {
	span.End()
}
