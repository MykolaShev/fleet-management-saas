package domain

import "github.com/google/uuid"

// FeatureFlagKey enumerates the toggleable features in the system. Keeping
// this a closed set (rather than an arbitrary string) means a typo'd or
// made-up key gets rejected by the service layer instead of silently
// creating a flag that nothing in the system ever actually checks.
type FeatureFlagKey string

const (
	FeatureAIAgent                FeatureFlagKey = "ai_agent"
	FeatureLiveTracking           FeatureFlagKey = "live_tracking"
	FeatureTelegramNotifications  FeatureFlagKey = "telegram_notifications"
)

// KnownFeatureFlags is the full catalog this system understands. Used both
// to validate incoming keys and to report every flag's state to a tenant
// that has never explicitly configured it (see FeatureFlagService.List).
var KnownFeatureFlags = []FeatureFlagKey{
	FeatureAIAgent,
	FeatureLiveTracking,
	FeatureTelegramNotifications,
}

func IsKnownFeatureFlag(key FeatureFlagKey) bool {
	for _, k := range KnownFeatureFlags {
		if k == key {
			return true
		}
	}
	return false
}

// FeatureFlag records whether a specific feature is turned on for a
// tenant. A row only exists once a tenant has explicitly set a flag —
// an absent row means "not configured, defaults to disabled".
type FeatureFlag struct {
	BaseModel
	TenantID uuid.UUID      `gorm:"type:uuid;not null;index:idx_feature_flags_tenant_key,unique" json:"tenant_id"`
	Key      FeatureFlagKey `gorm:"type:varchar(50);not null;index:idx_feature_flags_tenant_key,unique" json:"key"`
	Enabled  bool           `gorm:"not null;default:false" json:"enabled"`

	Tenant Tenant `gorm:"foreignKey:TenantID" json:"-"`
}

func (FeatureFlag) TableName() string { return "feature_flags" }