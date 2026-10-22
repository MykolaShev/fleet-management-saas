package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"

	"github.com/MykolaShev/fleet-management-saas/internal/domain"
)

// vehicleGetter, deliveryGetter and deliveryStatusUpdater are narrow,
// unexported interfaces over domain types only — the same narrow-
// interface pattern used everywhere else in this project (see
// internal/service), and the reason this package can depend only on
// internal/domain: *service.VehicleService and *service.DeliveryService
// already satisfy these via their existing Get/UpdateStatus methods, no
// adapter type needed on the service side beyond UpdateStatus itself.
type vehicleGetter interface {
	Get(ctx context.Context, tenantID, id uuid.UUID) (*domain.Vehicle, error)
}

type deliveryGetter interface {
	Get(ctx context.Context, tenantID, id uuid.UUID) (*domain.Delivery, error)
}

type deliveryStatusUpdater interface {
	UpdateStatus(ctx context.Context, tenantID, id uuid.UUID, status domain.DeliveryStatus) (*domain.Delivery, error)
}

// RegisterFleetTools wires this project's three tools into server. Each
// is a thin wrapper around an already-tested service method — no new
// business logic lives here, only argument parsing and honest
// "not available" framing for data the project doesn't have yet. 
func RegisterFleetTools(server *Server, vehicles vehicleGetter, deliveries deliveryGetter, statusUpdater deliveryStatusUpdater) {
	server.RegisterTool(ToolDefinition{
		Name:        "get_vehicle_location",
		Description: "Get the last reported GPS location of a vehicle by its ID. Returns an honest 'not yet reported' message if the vehicle has never sent a position.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"vehicle_id": map[string]any{"type": "string", "description": "UUID of the vehicle"},
			},
			"required": []string{"vehicle_id"},
		},
	}, getVehicleLocationHandler(vehicles))

	server.RegisterTool(ToolDefinition{
		Name:        "get_delivery_eta",
		Description: "Get the estimated arrival time for a delivery by its ID. This project has no live GPS tracking yet, so this always returns an honest 'not available' stub rather than a fabricated estimate.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"delivery_id": map[string]any{"type": "string", "description": "UUID of the delivery"},
			},
			"required": []string{"delivery_id"},
		},
	}, getDeliveryETAHandler(deliveries))

	server.RegisterTool(ToolDefinition{
		Name:        "update_delivery_status",
		Description: "Update a delivery's status by its ID.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"delivery_id": map[string]any{"type": "string", "description": "UUID of the delivery"},
				"status":      map[string]any{"type": "string", "enum": []string{"pending", "assigned", "in_transit", "delivered", "failed"}},
			},
			"required": []string{"delivery_id", "status"},
		},
	}, updateDeliveryStatusHandler(statusUpdater))
}

func getVehicleLocationHandler(vehicles vehicleGetter) ToolHandler {
	return func(ctx context.Context, tenantID uuid.UUID, arguments json.RawMessage) (string, error) {
		var args struct {
			VehicleID string `json:"vehicle_id"`
		}
		if err := json.Unmarshal(arguments, &args); err != nil {
			return "", fmt.Errorf("invalid arguments: %w", err)
		}
		id, err := uuid.Parse(args.VehicleID)
		if err != nil {
			return "", fmt.Errorf("vehicle_id must be a UUID: %w", err)
		}

		v, err := vehicles.Get(ctx, tenantID, id)
		if err != nil {
			return "", err
		}

		if v.CurrentLat == nil || v.CurrentLng == nil {
			return fmt.Sprintf("Vehicle %s (%s) has not yet reported a GPS location.", v.PlateNumber, v.ID), nil
		}
		return fmt.Sprintf("Vehicle %s (%s) was last seen at lat=%.6f, lng=%.6f.", v.PlateNumber, v.ID, *v.CurrentLat, *v.CurrentLng), nil
	}
}

func getDeliveryETAHandler(deliveries deliveryGetter) ToolHandler {
	return func(ctx context.Context, tenantID uuid.UUID, arguments json.RawMessage) (string, error) {
		var args struct {
			DeliveryID string `json:"delivery_id"`
		}
		if err := json.Unmarshal(arguments, &args); err != nil {
			return "", fmt.Errorf("invalid arguments: %w", err)
		}
		id, err := uuid.Parse(args.DeliveryID)
		if err != nil {
			return "", fmt.Errorf("delivery_id must be a UUID: %w", err)
		}

		d, err := deliveries.Get(ctx, tenantID, id)
		if err != nil {
			return "", err
		}

		// domain.Delivery has no ETA field on purpose (see the comment
		// on that struct) — an honest stub here, not a fabricated
		// estimate, until C10 (live GPS tracking) actually exists.
		return fmt.Sprintf("ETA for delivery %s is not available yet — it requires live GPS tracking, which this project doesn't have. Current status: %s.", d.ID, d.Status), nil
	}
}

func updateDeliveryStatusHandler(statusUpdater deliveryStatusUpdater) ToolHandler {
	return func(ctx context.Context, tenantID uuid.UUID, arguments json.RawMessage) (string, error) {
		var args struct {
			DeliveryID string `json:"delivery_id"`
			Status     string `json:"status"`
		}
		if err := json.Unmarshal(arguments, &args); err != nil {
			return "", fmt.Errorf("invalid arguments: %w", err)
		}
		id, err := uuid.Parse(args.DeliveryID)
		if err != nil {
			return "", fmt.Errorf("delivery_id must be a UUID: %w", err)
		}

		d, err := statusUpdater.UpdateStatus(ctx, tenantID, id, domain.DeliveryStatus(args.Status))
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("Delivery %s status updated to %s.", d.ID, d.Status), nil
	}
}