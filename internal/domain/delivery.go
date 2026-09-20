package domain

import (
	"time"

	"github.com/google/uuid"
)

// DeliveryStatus tracks a delivery through its lifecycle.
type DeliveryStatus string

const (
	DeliveryPending   DeliveryStatus = "pending"
	DeliveryAssigned  DeliveryStatus = "assigned"
	DeliveryInTransit DeliveryStatus = "in_transit"
	DeliveryDelivered DeliveryStatus = "delivered"
	DeliveryFailed    DeliveryStatus = "failed"
)

// Delivery represents a single delivery order within a tenant. VehicleID
// and DriverID are nullable because a delivery can exist (pending) before
// it's assigned to a vehicle/driver.
type Delivery struct {
	BaseModel
	TenantID  uuid.UUID  `gorm:"type:uuid;not null;index" json:"tenant_id"`
	VehicleID *uuid.UUID `gorm:"type:uuid;index" json:"vehicle_id"`
	DriverID  *uuid.UUID `gorm:"type:uuid;index" json:"driver_id"`

	Status DeliveryStatus `gorm:"type:varchar(20);not null;default:pending" json:"status"`

	PickupAddress  string  `gorm:"not null" json:"pickup_address"`
	PickupLat      float64 `json:"pickup_lat"`
	PickupLng      float64 `json:"pickup_lng"`
	DropoffAddress string  `gorm:"not null" json:"dropoff_address"`
	DropoffLat     float64 `json:"dropoff_lat"`
	DropoffLng     float64 `json:"dropoff_lng"`

	// ETA - estimated time of arrival
	ETA         *time.Time `json:"eta"`
	DeliveredAt *time.Time `json:"delivered_at"`

	// PhotoURL is set once the delivery-proof photo has been uploaded and
	// processed (Lambda + Blob/CDN stages).
	PhotoURL *string `json:"photo_url"`

	Tenant  Tenant   `gorm:"foreignKey:TenantID" json:"-"`
	Vehicle *Vehicle `gorm:"foreignKey:VehicleID" json:"-"`
	Driver  *User    `gorm:"foreignKey:DriverID" json:"-"`
}

func (Delivery) TableName() string { return "deliveries" }