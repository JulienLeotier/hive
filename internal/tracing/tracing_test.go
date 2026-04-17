package tracing

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

func TestSetupNoopWhenDisabled(t *testing.T) {
	// No endpoint, Enabled=false → should install no-op tracing (the default
	// otel provider) and return a no-op shutdown. Code that uses
	// otel.Tracer() must still produce usable, non-recording spans.
	shutdown, err := Setup(context.Background(), Config{})
	require.NoError(t, err)
	defer shutdown(context.Background())

	_, span := otel.Tracer("hive/test").Start(context.Background(), "demo")
	defer span.End()

	// No-op provider produces spans that are NOT recording.
	assert.False(t, span.IsRecording(),
		"disabled tracing should hand out non-recording spans — saves cycles in hot paths")
}

func TestSetupWithEndpointInstallsRealProvider(t *testing.T) {
	// Pointing at a bogus endpoint still wires the batch pipeline; the
	// exporter just won't succeed. The important observation is that spans
	// are now recording and carry the service.name resource attribute.
	shutdown, err := Setup(context.Background(), Config{
		Endpoint:       "127.0.0.1:4317",
		Protocol:       "grpc",
		SampleRatio:    1.0,
		ServiceVersion: "test",
	})
	require.NoError(t, err)
	defer shutdown(context.Background())

	_, span := otel.Tracer("hive/test").Start(context.Background(), "with-provider")
	defer span.End()

	assert.True(t, span.IsRecording(),
		"enabled tracing should produce recording spans")
	sc := span.SpanContext()
	assert.NotEqual(t, trace.TraceID{}, sc.TraceID(), "trace id must be set")
}

func TestSetupRejectsUnknownProtocol(t *testing.T) {
	_, err := Setup(context.Background(), Config{
		Endpoint: "127.0.0.1:4317",
		Protocol: "carrier-pigeon",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown OTLP protocol")
}
