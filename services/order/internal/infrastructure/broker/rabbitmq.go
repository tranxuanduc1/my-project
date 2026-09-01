package broker

import (
	"context"
	"encoding/json"
	"time"

	"myproject/order/internal/application/ports"

	"github.com/google/uuid"
	amqp "github.com/rabbitmq/amqp091-go"
	"gorm.io/datatypes"
)

type RabbitMQ struct {
	url string
}

func NewRabbitMQ(url string) *RabbitMQ { return &RabbitMQ{url: url} }

func (r *RabbitMQ) StartOutboxPublisher(ctx context.Context, store ports.OutboxStore) {
	for {
		cn, ch, err := r.open()
		if err != nil {
			time.Sleep(3 * time.Second)
			continue
		}
		for {
			events, err := store.PendingOutbox(ctx, 50)
			if err != nil {
				break
			}
			for _, event := range events {
				err = r.publish(ctx, ch, event.EventType, event.ID, event.Payload)
				if err == nil {
					_ = store.MarkOutboxPublished(ctx, event.ID)
				}
			}
			if err != nil {
				break
			}
			time.Sleep(500 * time.Millisecond)
		}
		ch.Close()
		cn.Close()
	}
}

func (r *RabbitMQ) StartPaymentConsumer(ctx context.Context, handler ports.PaymentEventHandler) {
	for {
		cn, ch, err := r.open()
		if err != nil {
			time.Sleep(3 * time.Second)
			continue
		}
		q, _ := ch.QueueDeclare("order.payment-status", true, false, false, false, amqp.Table{"x-dead-letter-exchange": "commerce.dead"})
		ch.QueueBind(q.Name, "payment.*", "commerce.events", false, nil)
		ch.Qos(10, 0, false)
		msgs, err := ch.Consume(q.Name, "order", false, false, false, false, nil)
		if err != nil {
			cn.Close()
			continue
		}
		for msg := range msgs {
			var event struct {
				EventID   uuid.UUID `json:"event_id"`
				OrderID   uuid.UUID `json:"order_id"`
				EventType string    `json:"event_type"`
			}
			if json.Unmarshal(msg.Body, &event) != nil {
				msg.Nack(false, false)
				continue
			}
			if err = handler.ApplyPayment(ctx, event.EventID, event.OrderID, event.EventType); err != nil {
				msg.Nack(false, true)
			} else {
				msg.Ack(false)
			}
		}
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
	return ch.PublishWithContext(ctx, "commerce.events", eventType, false, false, amqp.Publishing{DeliveryMode: amqp.Persistent, ContentType: "application/json", MessageId: id.String(), Body: payload})
}
