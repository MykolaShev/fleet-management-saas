// Command worker is B8's consumer process — deliberately a separate
// binary, not a goroutine inside cmd/api, so a slow/unavailable Telegram
// API can never block an HTTP request or the AI agent's tool call that
// triggered the status change. Run alongside the API:
//
//	go run ./cmd/worker
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/MykolaShev/fleet-management-saas/internal/config"
	"github.com/MykolaShev/fleet-management-saas/internal/queue"
	"github.com/MykolaShev/fleet-management-saas/internal/telegram"
)

func main() {
	cfg := config.Load()

	conn, err := amqp.Dial(cfg.RabbitMQURL)
	if err != nil {
		log.Fatalf("worker: connect to RabbitMQ at %s: %v", cfg.RabbitMQURL, err)
	}
	defer conn.Close()

	ch, err := conn.Channel()
	if err != nil {
		log.Fatalf("worker: open channel: %v", err)
	}
	defer ch.Close()

	if _, err := ch.QueueDeclare(queue.DeliveryNotificationsQueue, true, false, false, false, nil); err != nil {
		log.Fatalf("worker: declare queue: %v", err)
	}

	// autoAck=false: a message is only removed from the queue once we
	// confirm we actually handled it (Ack/Nack below) — a worker crash
	// mid-send leaves the message to be redelivered, not silently lost.
	msgs, err := ch.Consume(queue.DeliveryNotificationsQueue, "", false, false, false, false, nil)
	if err != nil {
		log.Fatalf("worker: start consuming: %v", err)
	}

	tg := telegram.NewClient(cfg.TelegramBotToken, cfg.TelegramChatID)

	log.Println("worker: waiting for delivery status-change notifications...")
	for msg := range msgs {
		var job queue.Job
		if err := json.Unmarshal(msg.Body, &job); err != nil {
			log.Printf("worker: malformed job, discarding: %v", err)
			_ = msg.Nack(false, false) // don't requeue — it'll never parse
			continue
		}

		text := fmt.Sprintf("Delivery %s: %s → %s", job.DeliveryID, job.OldStatus, job.NewStatus)
		if err := tg.SendMessage(context.Background(), text); err != nil {
			log.Printf("worker: telegram send failed, requeueing: %v", err)
			_ = msg.Nack(false, true) // transient (network/API hiccup) — try again
			continue
		}

		_ = msg.Ack(false)
		log.Printf("worker: notified Telegram for delivery %s", job.DeliveryID)
	}
}