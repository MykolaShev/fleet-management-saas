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

	// OAuthSubject stores the identity provider's unique subject id (the
	// Google "sub" claim), populated by the OAuth2 login flow. It's a
	// GLOBAL unique key, not scoped to a tenant: a Google account carries
	// no organization claim, so a login is identified purely by this
	// subject, independent of which tenant it ends up belonging to (see
	// AuthService.HandleGoogleLogin). The column name is pinned
	// explicitly rather than left to GORM's camelCase-to-snake_case
	// guess — "OAuth" is exactly the kind of mixed-case prefix that
	// convention is ambiguous about.
	OAuthSubject *string `gorm:"column:oauth_subject;uniqueIndex" json:"-"`

	Tenant Tenant `gorm:"foreignKey:TenantID" json:"-"`
}

func (User) TableName() string { return "users" }