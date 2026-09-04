package broker

import (
	"context"

	"myproject/payment/internal/application/ports"

	"github.com/google/uuid"
	amqp "github.com/rabbitmq/amqp091-go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
	"gorm.io/datatypes"
)

var (
	brokerTracer             = otel.Tracer("myproject/payment/broker")
	brokerMeter              = otel.Meter("myproject/payment/broker")
	publishAttempts          metric.Int64Counter
	consumerProcessed        metric.Int64Counter
	consumerAcknowledgements metric.Int64Counter
	reconnects               metric.Int64Counter
	consumerDuration         metric.Float64Histogram
	outboxPending            metric.Int64ObservableGauge
	outboxOldestAge          metric.Float64ObservableGauge
)

func init() {
	publishAttempts, _ = brokerMeter.Int64Counter("messaging.outbox.publish.attempts")
	consumerProcessed, _ = brokerMeter.Int64Counter("messaging.consumer.processed")
	consumerAcknowledgements, _ = brokerMeter.Int64Counter("messaging.consumer.acknowledgements")
	reconnects, _ = brokerMeter.Int64Counter("messaging.reconnects")
	consumerDuration, _ = brokerMeter.Float64Histogram("messaging.consumer.processing.duration", metric.WithUnit("s"))
	outboxPending, _ = brokerMeter.Int64ObservableGauge("outbox.pending")
	outboxOldestAge, _ = brokerMeter.Float64ObservableGauge("outbox.oldest_event_age", metric.WithUnit("s"))
}

func registerOutboxMetrics(store ports.OutboxStore) (metric.Registration, error) {
	return brokerMeter.RegisterCallback(func(ctx context.Context, observer metric.Observer) error {
		pending, oldestAge, err := store.OutboxStats(ctx)
		if err != nil {
			return err
		}
		observer.ObserveInt64(outboxPending, pending, metric.WithAttributes(attribute.String("service", "payment")))
		observer.ObserveFloat64(outboxOldestAge, oldestAge.Seconds(), metric.WithAttributes(attribute.String("service", "payment")))
		return nil
	}, outboxPending, outboxOldestAge)
}

func outboxContext(ctx context.Context, headers datatypes.JSONMap) context.Context {
	return otel.GetTextMapPropagator().Extract(ctx, jsonMapCarrier(headers))
}

func injectPublishingHeaders(ctx context.Context) amqp.Table {
	headers := amqp.Table{}
	otel.GetTextMapPropagator().Inject(ctx, amqpTableCarrier(headers))
	return headers
}

func messageContext(ctx context.Context, headers amqp.Table) context.Context {
	return otel.GetTextMapPropagator().Extract(ctx, amqpTableCarrier(headers))
}

func startProducerSpan(ctx context.Context, eventType string, eventID uuid.UUID) (context.Context, trace.Span) {
	return brokerTracer.Start(ctx, "publish "+eventType,
		trace.WithSpanKind(trace.SpanKindProducer),
		trace.WithAttributes(
			attribute.String("messaging.system", "rabbitmq"),
			attribute.String("messaging.destination.name", "commerce.events"),
			attribute.String("messaging.rabbitmq.routing_key", eventType),
			attribute.String("messaging.operation", "publish"),
			attribute.String("messaging.message.id", eventID.String()),
			attribute.String("event.type", eventType),
		),
	)
}

func startConsumerSpan(ctx context.Context, queue, routingKey, messageID string) (context.Context, trace.Span) {
	return brokerTracer.Start(ctx, "process "+routingKey,
		trace.WithSpanKind(trace.SpanKindConsumer),
		trace.WithAttributes(
			attribute.String("messaging.system", "rabbitmq"),
			attribute.String("messaging.destination.name", "commerce.events"),
			attribute.String("messaging.destination.kind", "topic"),
			attribute.String("messaging.rabbitmq.routing_key", routingKey),
			attribute.String("messaging.operation", "process"),
			attribute.String("messaging.consumer.queue", queue),
			attribute.String("messaging.message.id", messageID),
			attribute.String("event.type", routingKey),
		),
	)
}

func recordSpanError(span trace.Span, err error) {
	if err == nil {
		return
	}
	span.RecordError(err)
	span.SetStatus(codes.Error, err.Error())
}

type jsonMapCarrier datatypes.JSONMap

func (c jsonMapCarrier) Get(key string) string {
	if value, ok := c[key]; ok {
		if text, ok := value.(string); ok {
			return text
		}
	}
	return ""
}

func (c jsonMapCarrier) Set(key, value string) { c[key] = value }

func (c jsonMapCarrier) Keys() []string {
	keys := make([]string, 0, len(c))
	for key := range c {
		keys = append(keys, key)
	}
	return keys
}

type amqpTableCarrier amqp.Table

func (c amqpTableCarrier) Get(key string) string {
	if value, ok := c[key]; ok {
		if text, ok := value.(string); ok {
			return text
		}
		if bytes, ok := value.([]byte); ok {
			return string(bytes)
		}
	}
	return ""
}

func (c amqpTableCarrier) Set(key, value string) { c[key] = value }

func (c amqpTableCarrier) Keys() []string {
	keys := make([]string, 0, len(c))
	for key := range c {
		keys = append(keys, key)
	}
	return keys
}

var _ propagation.TextMapCarrier = jsonMapCarrier{}
var _ propagation.TextMapCarrier = amqpTableCarrier{}
