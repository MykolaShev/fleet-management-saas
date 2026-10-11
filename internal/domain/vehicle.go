package domain

import (
	"time"

	"github.com/google/uuid"
)

// VehicleStatus reflects what a vehicle is currently doing.
type VehicleStatus string

const (
	VehicleIdle        VehicleStatus = "idle"
	VehicleEnRoute     VehicleStatus = "en_route"
	VehicleMaintenance VehicleStatus = "maintenance"
)

// Vehicle belongs to a tenant's fleet. CurrentLat/CurrentLng are updated by
// the live-tracking stage (WebSockets); they start out nil until a vehicle
// reports its first position.
type Vehicle struct {
	BaseModel
	TenantID    uuid.UUID     `gorm:"type:uuid;not null;index" json:"tenant_id"`
	PlateNumber string        `gorm:"not null" json:"plate_number"`
	Model       string        `json:"model"`
	Status      VehicleStatus `gorm:"type:varchar(20);not null;default:idle" json:"status"`

	CurrentLat     *float64   `json:"current_lat"`
	CurrentLng     *float64   `json:"current_lng"`
	LastLocationAt *time.Time `json:"last_location_at"`

	Tenant Tenant `gorm:"foreignKey:TenantID" json:"-"`
}

func (Vehicle) TableName() string { return "vehicles" }