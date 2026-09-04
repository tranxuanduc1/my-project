package broker

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"myproject/order/internal/application/ports"

	"github.com/google/uuid"
	amqp "github.com/rabbitmq/amqp091-go"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"gorm.io/datatypes"
)

type RabbitMQ struct {
	url string
}

func NewRabbitMQ(url string) *RabbitMQ { return &RabbitMQ{url: url} }

func (r *RabbitMQ) StartOutboxPublisher(ctx context.Context, store ports.OutboxStore) {
	registration, err := registerOutboxMetrics(store)
	if err != nil {
		slog.WarnContext(ctx, "order outbox metrics registration failed", "error", err)
	}
	defer func() {
		if registration != nil {
			_ = registration.Unregister()
		}
	}()
	for {
		if ctx.Err() != nil {
			return
		}
		cn, ch, err := r.open()
		if err != nil {
			reconnects.Add(ctx, 1, metric.WithAttributes(attribute.String("service", "order"), attribute.String("operation", "outbox_publish"), attribute.String("result", "connect_failed")))
			slog.WarnContext(ctx, "order outbox publisher connect failed", "error", err)
			if !sleep(ctx, 3*time.Second) {
				return
			}
			continue
		}
		slog.InfoContext(ctx, "order outbox publisher connected")
		for {
			events, err := store.PendingOutbox(ctx, 50)
			if err != nil {
				if ctx.Err() != nil {
					ch.Close()
					cn.Close()
					return
				}
				slog.ErrorContext(ctx, "order outbox fetch failed", "error", err)
				break
			}
			for _, event := range events {
				eventCtx, span := startProducerSpan(outboxContext(ctx, event.Headers), event.EventType, event.ID)
				err = r.publish(eventCtx, ch, event.EventType, event.ID, event.Payload)
				if err == nil {
					publishAttempts.Add(eventCtx, 1, metric.WithAttributes(attribute.String("service", "order"), attribute.String("event_type", event.EventType), attribute.String("result", "success")))
					if markErr := store.MarkOutboxPublished(eventCtx, event.ID); markErr != nil {
						recordSpanError(span, markErr)
						slog.ErrorContext(eventCtx, "order outbox mark published failed", "event_id", event.ID, "event_type", event.EventType, "error", markErr)
						span.End()
						continue
					}
					slog.InfoContext(eventCtx, "order outbox event published", "event_id", event.ID, "event_type", event.EventType)
				} else {
					publishAttempts.Add(eventCtx, 1, metric.WithAttributes(attribute.String("service", "order"), attribute.String("event_type", event.EventType), attribute.String("result", "failure")))
					recordSpanError(span, err)
					slog.ErrorContext(eventCtx, "order outbox publish failed", "event_id", event.ID, "event_type", event.EventType, "error", err)
				}
				span.End()
			}
			if err != nil {
				break
			}
			if !sleep(ctx, 500*time.Millisecond) {
				ch.Close()
				cn.Close()
				return
			}
		}
		slog.WarnContext(ctx, "order outbox publisher disconnected")
		reconnects.Add(ctx, 1, metric.WithAttributes(attribute.String("service", "order"), attribute.String("operation", "outbox_publish"), attribute.String("result", "disconnected")))
		ch.Close()
		cn.Close()
	}
}

func (r *RabbitMQ) StartPaymentConsumer(ctx context.Context, handler ports.PaymentEventHandler) {
	for {
		if ctx.Err() != nil {
			return
		}
		cn, ch, err := r.open()
		if err != nil {
			reconnects.Add(ctx, 1, metric.WithAttributes(attribute.String("service", "order"), attribute.String("operation", "consume_payment_status"), attribute.String("result", "connect_failed")))
			slog.WarnContext(ctx, "payment consumer connect failed", "error", err)
			if !sleep(ctx, 3*time.Second) {
				return
			}
			continue
		}
		slog.InfoContext(ctx, "payment consumer connected")
		done := make(chan struct{})
		go func() {
			select {
			case <-ctx.Done():
				cn.Close()
			case <-done:
			}
		}()
		q, err := ch.QueueDeclare("order.payment-status", true, false, false, false, amqp.Table{"x-dead-letter-exchange": "commerce.dead"})
		if err != nil {
			slog.ErrorContext(ctx, "payment consumer queue declare failed", "error", err)
			close(done)
			cn.Close()
			continue
		}
		if err := ch.QueueBind(q.Name, "payment.*", "commerce.events", false, nil); err != nil {
			slog.ErrorContext(ctx, "payment consumer queue bind failed", "queue", q.Name, "error", err)
			close(done)
			cn.Close()
			continue
		}
		if err := ch.Qos(10, 0, false); err != nil {
			slog.ErrorContext(ctx, "payment consumer qos failed", "queue", q.Name, "error", err)
			close(done)
			cn.Close()
			continue
		}
		msgs, err := ch.Consume(q.Name, "order", false, false, false, false, nil)
		if err != nil {
			slog.ErrorContext(ctx, "payment consumer start failed", "queue", q.Name, "error", err)
			close(done)
			cn.Close()
			continue
		}
		for msg := range msgs {
			startedAt := time.Now()
			msgCtx, span := startConsumerSpan(messageContext(ctx, msg.Headers), q.Name, msg.RoutingKey, msg.MessageId)
			if ctx.Err() != nil {
				span.End()
				close(done)
				cn.Close()
				return
			}
			var event struct {
				EventID   uuid.UUID `json:"event_id"`
				OrderID   uuid.UUID `json:"order_id"`
				EventType string    `json:"event_type"`
			}
			if json.Unmarshal(msg.Body, &event) != nil {
				consumerProcessed.Add(msgCtx, 1, metric.WithAttributes(attribute.String("service", "order"), attribute.String("event_type", msg.RoutingKey), attribute.String("result", "decode_failure")))
				consumerDuration.Record(msgCtx, time.Since(startedAt).Seconds(), metric.WithAttributes(attribute.String("service", "order"), attribute.String("event_type", msg.RoutingKey), attribute.String("result", "decode_failure")))
				slog.WarnContext(msgCtx, "payment event rejected", "reason", "invalid json", "message_id", msg.MessageId)
				_ = msg.Nack(false, false)
				consumerAcknowledgements.Add(msgCtx, 1, metric.WithAttributes(attribute.String("service", "order"), attribute.String("operation", "nack"), attribute.String("requeue", "false")))
				span.SetStatus(codes.Error, "invalid json")
				span.End()
				continue
			}
			span.SetAttributes(attribute.String("event.id", event.EventID.String()), attribute.String("order.id", event.OrderID.String()), attribute.String("event.type", event.EventType))
			if err = handler.ApplyPayment(msgCtx, event.EventID, event.OrderID, event.EventType); err != nil {
				recordSpanError(span, err)
				consumerProcessed.Add(msgCtx, 1, metric.WithAttributes(attribute.String("service", "order"), attribute.String("event_type", event.EventType), attribute.String("result", "handler_failure")))
				consumerDuration.Record(msgCtx, time.Since(startedAt).Seconds(), metric.WithAttributes(attribute.String("service", "order"), attribute.String("event_type", event.EventType), attribute.String("result", "handler_failure")))
				slog.WarnContext(msgCtx, "payment event nacked", "event_id", event.EventID, "order_id", event.OrderID, "event_type", event.EventType, "requeue", true)
				_ = msg.Nack(false, true)
				consumerAcknowledgements.Add(msgCtx, 1, metric.WithAttributes(attribute.String("service", "order"), attribute.String("operation", "nack"), attribute.String("requeue", "true")))
			} else {
				consumerProcessed.Add(msgCtx, 1, metric.WithAttributes(attribute.String("service", "order"), attribute.String("event_type", event.EventType), attribute.String("result", "success")))
				consumerDuration.Record(msgCtx, time.Since(startedAt).Seconds(), metric.WithAttributes(attribute.String("service", "order"), attribute.String("event_type", event.EventType), attribute.String("result", "success")))
				slog.InfoContext(msgCtx, "payment event acknowledged", "event_id", event.EventID, "order_id", event.OrderID, "event_type", event.EventType)
				_ = msg.Ack(false)
				consumerAcknowledgements.Add(msgCtx, 1, metric.WithAttributes(attribute.String("service", "order"), attribute.String("operation", "ack"), attribute.String("requeue", "false")))
			}
			span.End()
		}
		slog.WarnContext(ctx, "payment consumer disconnected")
		reconnects.Add(ctx, 1, metric.WithAttributes(attribute.String("service", "order"), attribute.String("operation", "consume_payment_status"), attribute.String("result", "disconnected")))
		close(done)
		cn.Close()
	}
}

func (r *RabbitMQ) Publish(ctx context.Context, eventType string, id uuid.UUID, payload datatypes.JSON) error {
	cn, ch, err := r.open()
	if err != nil {
		return err
	}
	defer cn.Close()
	defer ch.Close()
	return r.publish(ctx, ch, eventType, id, payload)
}

func (r *RabbitMQ) open() (*amqp.Connection, *amqp.Channel, error) {
	cn, err := amqp.Dial(r.url)
	if err != nil {
		return nil, nil, err
	}
	ch, err := cn.Channel()
	if err != nil {
		cn.Close()
		return nil, nil, err
	}
	ch.ExchangeDeclare("commerce.events", "topic", true, false, false, false, nil)
	return cn, ch, nil
}

func (r *RabbitMQ) publish(ctx context.Context, ch *amqp.Channel, eventType string, id uuid.UUID, payload datatypes.JSON) error {
	return ch.PublishWithContext(ctx, "commerce.events", eventType, false, false, amqp.Publishing{DeliveryMode: amqp.Persistent, ContentType: "application/json", MessageId: id.String(), Headers: injectPublishingHeaders(ctx), Body: payload})
}

func sleep(ctx context.Context, d time.Duration) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
