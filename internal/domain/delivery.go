package domain

import "github.com/google/uuid"

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
//
// Kept intentionally minimal for the CRUD stage: geocoordinates, ETA and
// photo-proof fields are added later, exactly when the stage that needs
// them (AI agent / Lambda+CDN) is implemented, rather than sitting unused
// from day one.
type Delivery struct {
	BaseModel
	TenantID  uuid.UUID  `gorm:"type:uuid;not null;index" json:"tenant_id"`
	VehicleID *uuid.UUID `gorm:"type:uuid;index" json:"vehicle_id"`
	DriverID  *uuid.UUID `gorm:"type:uuid;index" json:"driver_id"`

	Status DeliveryStatus `gorm:"type:varchar(20);not null;default:pending" json:"status"`

	PickupAddress  string `gorm:"not null" json:"pickup_address"`
	DropoffAddress string `gorm:"not null" json:"dropoff_address"`

	Tenant  Tenant   `gorm:"foreignKey:TenantID" json:"-"`
	Vehicle *Vehicle `gorm:"foreignKey:VehicleID" json:"-"`
	Driver  *User    `gorm:"foreignKey:DriverID" json:"-"`
}

func (Delivery) TableName() string { return "deliveries" }