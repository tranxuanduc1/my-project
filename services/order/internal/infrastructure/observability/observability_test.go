package observability

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"go.opentelemetry.io/otel/attribute"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

func TestResourceAttributes(t *testing.T) {
	res, err := newResource(Config{
		ServiceVersion:        "test-version",
		DeploymentEnvironment: "test",
		ServiceInstanceID:     "instance-1",
	})
	if err != nil {
		t.Fatal(err)
	}

	attrs := map[attribute.Key]string{}
	for _, kv := range res.Attributes() {
		attrs[kv.Key] = kv.Value.AsString()
	}

	assertAttr(t, attrs, "service.name", "order")
	assertAttr(t, attrs, "service.version", "test-version")
	assertAttr(t, attrs, "deployment.environment.name", "test")
	assertAttr(t, attrs, "service.instance.id", "instance-1")
}

func TestSetupShutdownIsIdempotent(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	providers, err := Setup(ctx, Config{
		OTLPEndpoint:          "http://127.0.0.1:4318",
		ServiceVersion:        "test",
		DeploymentEnvironment: "test",
		TraceSampler:          "parentbased_traceidratio",
		TraceSamplerArg:       "1.0",
		LogLevel:              "debug",
		ShutdownTimeout:       50 * time.Millisecond,
	}, bytes.NewBuffer(nil))
	if err != nil {
		t.Fatal(err)
	}
	if providers.Logger == nil || providers.LoggerProvider == nil || providers.TracerProvider == nil || providers.MeterProvider == nil {
		t.Fatal("expected logger, logger provider, tracer provider, and meter provider")
	}
	providers.Logger.Info("startup log over otel bridge")
	_ = providers.Shutdown(context.Background())
	if err := providers.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestLoggerAddsTraceCorrelation(t *testing.T) {
	var out bytes.Buffer
	loggerProvider := sdklog.NewLoggerProvider()
	logger := NewLogger(Config{LogLevel: "debug", ServiceVersion: "test"}, &out, loggerProvider)
	tracerProvider := sdktrace.NewTracerProvider()
	_, span := tracerProvider.Tracer("test").Start(context.Background(), "operation")
	defer span.End()

	logger.InfoContext(trace.ContextWithSpan(context.Background(), span), "correlated log")

	logLine := out.String()
	if !strings.Contains(logLine, `"service":"order"`) {
		t.Fatalf("expected service field in %q", logLine)
	}
	if !strings.Contains(logLine, `"trace_id":"`) || !strings.Contains(logLine, `"span_id":"`) {
		t.Fatalf("expected trace correlation fields in %q", logLine)
	}
}

func assertAttr(t *testing.T, attrs map[attribute.Key]string, key attribute.Key, want string) {
	t.Helper()
	if got := attrs[key]; got != want {
		t.Fatalf("attribute %s = %q, want %q", key, got, want)
	}
}
