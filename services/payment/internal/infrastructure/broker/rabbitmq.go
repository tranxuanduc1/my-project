package broker

import (
	"context"
	"encoding/json"
	"time"

	"myproject/payment/internal/application/ports"

	"github.com/google/uuid"
	amqp "github.com/rabbitmq/amqp091-go"
)

type RabbitMQ struct {
	url string
}

func NewRabbitMQ(url string) *RabbitMQ { return &RabbitMQ{url: url} }

func (r *RabbitMQ) StartOrderConsumer(ctx context.Context, handler ports.OrderCreatedHandler) {
	for {
		cn, ch, err := r.open()
		if err != nil {
			time.Sleep(3 * time.Second)
			continue
		}
		q, _ := ch.QueueDeclare("payment.order-created", true, false, false, false, amqp.Table{"x-dead-letter-exchange": "commerce.dead"})
		ch.QueueBind(q.Name, "order.created", "commerce.events", false, nil)
		ch.Qos(10, 0, false)
		msgs, err := ch.Consume(q.Name, "payment", false, false, false, false, nil)
		if err != nil {
			cn.Close()
			continue
		}
		for msg := range msgs {
			var event struct {
				EventID     uuid.UUID `json:"event_id"`
				OrderID     uuid.UUID `json:"order_id"`
				UserID      uuid.UUID `json:"user_id"`
				AmountCents int64     `json:"amount_cents"`
				Currency    string    `json:"currency"`
			}
			if json.Unmarshal(msg.Body, &event) != nil {
				msg.Nack(false, false)
				continue
			}
			if err = handler.CreateFromOrder(ctx, event.EventID, event.OrderID, event.UserID, event.AmountCents, event.Currency); err != nil {
				msg.Nack(false, true)
			} else {
				msg.Ack(false)
			}
		}
		cn.Close()
	}
}

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
				err = ch.PublishWithContext(ctx, "commerce.events", event.EventType, false, false, amqp.Publishing{DeliveryMode: amqp.Persistent, ContentType: "application/json", MessageId: event.ID.String(), Body: event.Payload})
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
	ch.ExchangeDeclare("commerce.dead", "topic", true, false, false, false, nil)
	return cn, ch, nil
}
