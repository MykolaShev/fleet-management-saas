package domain

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// BaseModel holds fields common to every persisted entity: a UUID primary
// key (generated in application code, not by the DB) and standard
// timestamps managed by GORM.
type BaseModel struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// BeforeCreate assigns a UUID before insert if one hasn't been set already.
// Because it's defined on BaseModel and every entity embeds it, GORM picks
// this hook up automatically via Go's method promotion.
func (b *BaseModel) BeforeCreate(tx *gorm.DB) error {
	if b.ID == uuid.Nil {
		b.ID = uuid.New()
	}
	return nil
}