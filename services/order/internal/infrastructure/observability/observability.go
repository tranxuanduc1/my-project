package observability

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"myproject/order/internal/infrastructure/config"

	"go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	otellogglobal "go.opentelemetry.io/otel/log/global"
	"go.opentelemetry.io/otel/propagation"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

const serviceName = "order"

type Config struct {
	OTLPEndpoint          string
	ServiceVersion        string
	DeploymentEnvironment string
	ServiceInstanceID     string
	TraceSampler          string
	TraceSamplerArg       string
	LogLevel              string
	ShutdownTimeout       time.Duration
}

type Providers struct {
	Logger         *slog.Logger
	LoggerProvider *sdklog.LoggerProvider
	TracerProvider *sdktrace.TracerProvider
	MeterProvider  *sdkmetric.MeterProvider
	shutdown       func(context.Context) error
}

func LoadConfig() Config {
	return Config{
		OTLPEndpoint:          config.Env("OTEL_EXPORTER_OTLP_ENDPOINT", "http://otel-collector:4318"),
		ServiceVersion:        config.Env("OTEL_SERVICE_VERSION", "dev"),
		DeploymentEnvironment: config.Env("OTEL_DEPLOYMENT_ENVIRONMENT", "local"),
		ServiceInstanceID:     serviceInstanceID(),
		TraceSampler:          config.Env("OTEL_TRACES_SAMPLER", "parentbased_traceidratio"),
		TraceSamplerArg:       config.Env("OTEL_TRACES_SAMPLER_ARG", "1.0"),
		LogLevel:              config.Env("LOG_LEVEL", "info"),
		ShutdownTimeout:       5 * time.Second,
	}
}

func Setup(ctx context.Context, cfg Config, out io.Writer) (*Providers, error) {
	if out == nil {
		out = os.Stdout
	}
	res, err := newResource(cfg)
	if err != nil {
		return nil, err
	}
	logExporter, err := otlploghttp.New(ctx, logHTTPOptions(cfg.OTLPEndpoint)...)
	if err != nil {
		return nil, err
	}
	traceExporter, err := otlptracehttp.New(ctx, traceHTTPOptions(cfg.OTLPEndpoint)...)
	if err != nil {
		return nil, err
	}
	metricExporter, err := otlpmetrichttp.New(ctx, metricHTTPOptions(cfg.OTLPEndpoint)...)
	if err != nil {
		return nil, err
	}

	loggerProvider := sdklog.NewLoggerProvider(
		sdklog.WithProcessor(sdklog.NewBatchProcessor(logExporter)),
		sdklog.WithResource(res),
	)
	tracerProvider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(traceExporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(traceSampler(cfg)),
	)
	meterProvider := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metricExporter)),
		sdkmetric.WithResource(res),
	)

	otel.SetTracerProvider(tracerProvider)
	otel.SetMeterProvider(meterProvider)
	otellogglobal.SetLoggerProvider(loggerProvider)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))
	logger := NewLogger(cfg, out, loggerProvider)

	var once sync.Once
	providers := &Providers{
		Logger:         logger,
		LoggerProvider: loggerProvider,
		TracerProvider: tracerProvider,
		MeterProvider:  meterProvider,
	}
	providers.shutdown = func(ctx context.Context) error {
		var err error
		once.Do(func() {
			if _, ok := ctx.Deadline(); !ok && cfg.ShutdownTimeout > 0 {
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(ctx, cfg.ShutdownTimeout)
				defer cancel()
			}
			err = errors.Join(
				loggerProvider.Shutdown(ctx),
				tracerProvider.Shutdown(ctx),
				meterProvider.Shutdown(ctx),
			)
		})
		return err
	}
	return providers, nil
}

func NewLogger(cfg Config, out io.Writer, provider *sdklog.LoggerProvider) *slog.Logger {
	level := new(slog.LevelVar)
	level.Set(parseLogLevel(cfg.LogLevel))
	stdout := slog.NewJSONHandler(out, &slog.HandlerOptions{Level: level})
	otelHandler := otelslog.NewHandler(
		"myproject/order",
		otelslog.WithLoggerProvider(provider),
		otelslog.WithVersion(cfg.ServiceVersion),
	)
	return slog.New(levelHandler{
		level:   level,
		handler: traceContextHandler{handler: multiHandler{stdout, otelHandler}},
	}).With("service", serviceName)
}

func (p *Providers) Shutdown(ctx context.Context) error {
	if p == nil || p.shutdown == nil {
		return nil
	}
	return p.shutdown(ctx)
}

func newResource(cfg Config) (*resource.Resource, error) {
	attrs := []attribute.KeyValue{
		attribute.String("service.name", serviceName),
		attribute.String("service.version", cfg.ServiceVersion),
		attribute.String("deployment.environment.name", cfg.DeploymentEnvironment),
	}
	if cfg.ServiceInstanceID != "" {
		attrs = append(attrs, attribute.String("service.instance.id", cfg.ServiceInstanceID))
	}
	return resource.Merge(
		resource.Default(),
		resource.NewWithAttributes("", attrs...),
	)
}

func traceSampler(cfg Config) sdktrace.Sampler {
	switch strings.ToLower(strings.TrimSpace(cfg.TraceSampler)) {
	case "always_on":
		return sdktrace.AlwaysSample()
	case "always_off":
		return sdktrace.NeverSample()
	case "traceidratio":
		return sdktrace.TraceIDRatioBased(traceRatio(cfg.TraceSamplerArg))
	case "parentbased_always_on":
		return sdktrace.ParentBased(sdktrace.AlwaysSample())
	case "parentbased_always_off":
		return sdktrace.ParentBased(sdktrace.NeverSample())
	case "parentbased_traceidratio", "":
		return sdktrace.ParentBased(sdktrace.TraceIDRatioBased(traceRatio(cfg.TraceSamplerArg)))
	default:
		return sdktrace.ParentBased(sdktrace.TraceIDRatioBased(1.0))
	}
}

func traceRatio(value string) float64 {
	ratio, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	if err != nil {
		return 1.0
	}
	if ratio < 0 {
		return 0
	}
	if ratio > 1 {
		return 1
	}
	return ratio
}

func traceHTTPOptions(endpoint string) []otlptracehttp.Option {
	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return []otlptracehttp.Option{otlptracehttp.WithEndpointURL(endpoint)}
	}
	options := []otlptracehttp.Option{otlptracehttp.WithEndpoint(parsed.Host)}
	if parsed.Scheme == "http" {
		options = append(options, otlptracehttp.WithInsecure())
	}
	if parsed.Path != "" && parsed.Path != "/" {
		options = append(options, otlptracehttp.WithURLPath(strings.TrimRight(parsed.Path, "/")+"/v1/traces"))
	}
	return options
}

func metricHTTPOptions(endpoint string) []otlpmetrichttp.Option {
	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return []otlpmetrichttp.Option{otlpmetrichttp.WithEndpointURL(endpoint)}
	}
	options := []otlpmetrichttp.Option{otlpmetrichttp.WithEndpoint(parsed.Host)}
	if parsed.Scheme == "http" {
		options = append(options, otlpmetrichttp.WithInsecure())
	}
	if parsed.Path != "" && parsed.Path != "/" {
		options = append(options, otlpmetrichttp.WithURLPath(strings.TrimRight(parsed.Path, "/")+"/v1/metrics"))
	}
	return options
}

func logHTTPOptions(endpoint string) []otlploghttp.Option {
	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return []otlploghttp.Option{otlploghttp.WithEndpointURL(endpoint)}
	}
	options := []otlploghttp.Option{otlploghttp.WithEndpoint(parsed.Host)}
	if parsed.Scheme == "http" {
		options = append(options, otlploghttp.WithInsecure())
	}
	if parsed.Path != "" && parsed.Path != "/" {
		options = append(options, otlploghttp.WithURLPath(strings.TrimRight(parsed.Path, "/")+"/v1/logs"))
	}
	return options
}

func serviceInstanceID() string {
	if value := os.Getenv("OTEL_SERVICE_INSTANCE_ID"); value != "" {
		return value
	}
	return os.Getenv("HOSTNAME")
}

func parseLogLevel(value string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

type multiHandler []slog.Handler

func (h multiHandler) Enabled(ctx context.Context, level slog.Level) bool {
	for _, handler := range h {
		if handler.Enabled(ctx, level) {
			return true
		}
	}
	return false
}

func (h multiHandler) Handle(ctx context.Context, record slog.Record) error {
	var err error
	for _, handler := range h {
		err = errors.Join(err, handler.Handle(ctx, record))
	}
	return err
}

func (h multiHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	handlers := make(multiHandler, 0, len(h))
	for _, handler := range h {
		handlers = append(handlers, handler.WithAttrs(attrs))
	}
	return handlers
}

func (h multiHandler) WithGroup(name string) slog.Handler {
	handlers := make(multiHandler, 0, len(h))
	for _, handler := range h {
		handlers = append(handlers, handler.WithGroup(name))
	}
	return handlers
}

type levelHandler struct {
	level   slog.Leveler
	handler slog.Handler
}

func (h levelHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return level >= h.level.Level() && h.handler.Enabled(ctx, level)
}

func (h levelHandler) Handle(ctx context.Context, record slog.Record) error {
	return h.handler.Handle(ctx, record)
}

func (h levelHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return levelHandler{level: h.level, handler: h.handler.WithAttrs(attrs)}
}

func (h levelHandler) WithGroup(name string) slog.Handler {
	return levelHandler{level: h.level, handler: h.handler.WithGroup(name)}
}

type traceContextHandler struct {
	handler slog.Handler
}

func (h traceContextHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.handler.Enabled(ctx, level)
}

func (h traceContextHandler) Handle(ctx context.Context, record slog.Record) error {
	if spanContext := trace.SpanContextFromContext(ctx); spanContext.IsValid() {
		record.AddAttrs(
			slog.String("trace_id", spanContext.TraceID().String()),
			slog.String("span_id", spanContext.SpanID().String()),
		)
	}
	return h.handler.Handle(ctx, record)
}

func (h traceContextHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return traceContextHandler{handler: h.handler.WithAttrs(attrs)}
}

func (h traceContextHandler) WithGroup(name string) slog.Handler {
	return traceContextHandler{handler: h.handler.WithGroup(name)}
}
