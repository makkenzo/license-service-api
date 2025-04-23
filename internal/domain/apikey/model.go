package apikey

import (
	"time"

	"github.com/google/uuid"
)

type APIKey struct {
	ID          uuid.UUID  `db:"id"`
	KeyHash     string     `db:"key_hash"`
	Prefix      string     `db:"prefix"`
	Description string     `db:"description"`
	ProductID   uuid.UUID  `db:"product_id"`
	IsEnabled   bool       `db:"is_enabled"`
	CreatedAt   time.Time  `db:"created_at"`
	LastUsedAt  *time.Time `db:"last_used_at"`
}

const (
	APIKeyPrefixLength = 8
	APIKeySecretLength = 32
	APIKeyFormat       = "lm_%s_%s"
)
