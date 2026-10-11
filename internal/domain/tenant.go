package domain

// Tenant represents a company using the platform. Every other domain
// entity (User, Vehicle, Delivery) belongs to exactly one tenant, and all
// data access must be scoped by TenantID (enforced at the repository layer
// as part of the tenant-isolation stage).
type Tenant struct {
	BaseModel
	Name string `gorm:"not null" json:"name"`

	Users      []User     `gorm:"foreignKey:TenantID" json:"-"`
	Vehicles   []Vehicle  `gorm:"foreignKey:TenantID" json:"-"`
	Deliveries []Delivery `gorm:"foreignKey:TenantID" json:"-"`
}

func (Tenant) TableName() string { return "tenants" }