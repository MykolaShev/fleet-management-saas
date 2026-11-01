package queue

import (
	"context"
	"encoding/json"
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"
)

// RabbitMQPublisher wraps one AMQP channel. Not covered by go test — that
// would need a live broker in CI; exercised via docker-compose + a real
// Telegram message arriving, same as cache's Redis backend.
type RabbitMQPublisher struct {
	ch *amqp.Channel
}

// NewRabbitMQPublisher opens a channel on conn and declares the durable
// queue so messages survive a broker restart — appropriate for "tell the
// driver's manager their delivery moved," not appropriate to silently
// drop.
func NewRabbitMQPublisher(conn *amqp.Connection) (*RabbitMQPublisher, error) {
	ch, err := conn.Channel()
	if err != nil {
		return nil, fmt.Errorf("open channel: %w", err)
	}
	if _, err := ch.QueueDeclare(DeliveryNotificationsQueue, true, false, false, false, nil); err != nil {
		return nil, fmt.Errorf("declare queue: %w", err)
	}
	return &RabbitMQPublisher{ch: ch}, nil
}

func (p *RabbitMQPublisher) Publish(ctx context.Context, job Job) error {
	body, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("marshal job: %w", err)
	}
	return p.ch.PublishWithContext(ctx, "", DeliveryNotificationsQueue, false, false, amqp.Publishing{
		ContentType:  "application/json",
		DeliveryMode: amqp.Persistent,
		Body:         body,
	})
}