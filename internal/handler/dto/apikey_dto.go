package dto

import (
	"time"

	"github.com/google/uuid"
)

type CreateAPIKeyRequest struct {
	Description string    `json:"description" binding:"required"`
	ProductID   uuid.UUID `json:"product_id,omitempty"`
}

type CreateAPIKeyResponse struct {
	ID          uuid.UUID `json:"id"`
	FullKey     string    `json:"full_key"`
	Prefix      string    `json:"prefix"`
	Description string    `json:"description"`
	ProductID   uuid.UUID `json:"product_id,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

type APIKeyResponse struct {
	ID          uuid.UUID  `json:"id"`
	Prefix      string     `json:"prefix"`
	Description string     `json:"description"`
	ProductID   uuid.UUID  `json:"product_id,omitempty"`
	IsEnabled   bool       `json:"is_enabled"`
	CreatedAt   time.Time  `json:"created_at"`
	LastUsedAt  *time.Time `json:"last_used_at,omitempty"`
}
