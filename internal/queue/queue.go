package queue

import (
	"context"

	"github.com/google/uuid"

	"github.com/MykolaShev/fleet-management-saas/internal/domain"
)

const DeliveryNotificationsQueue = "delivery_notifications"

// Job is one status-change event, queued for the worker to act on.
type Job struct {
	DeliveryID uuid.UUID             `json:"delivery_id"`
	OldStatus  domain.DeliveryStatus `json:"old_status"`
	NewStatus  domain.DeliveryStatus `json:"new_status"`
}

// Publisher is the narrow interface internal/service depends on — so
// tests can fake it without a live broker, and so service.DeliveryService
// never imports the RabbitMQ client directly.
type Publisher interface {
	Publish(ctx context.Context, job Job) error
}