package domain

import "github.com/google/uuid"

// UserRole distinguishes what a user is allowed to do within their tenant.
type UserRole string

const (
	RoleManager UserRole = "manager"
	RoleDriver  UserRole = "driver"
)

// User is a person belonging to a tenant — either a manager (back-office,
// assigns deliveries) or a driver (executes deliveries, updates status).
type User struct {
	BaseModel
	TenantID uuid.UUID `gorm:"type:uuid;not null;index:idx_users_tenant_email,unique" json:"tenant_id"`
	Email    string    `gorm:"not null;index:idx_users_tenant_email,unique" json:"email"`
	Name     string    `gorm:"not null" json:"name"`
	Role     UserRole  `gorm:"type:varchar(20);not null" json:"role"`

	// OAuthSubject stores the identity provider's unique subject/object id
	// (e.g. the Microsoft Entra ID "oid" claim), populated once OAuth2
	// login is wired up.
	OAuthSubject *string `gorm:"index" json:"-"`

	Tenant Tenant `gorm:"foreignKey:TenantID" json:"-"`
}

func (User) TableName() string { return "users" }