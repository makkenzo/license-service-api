package apikey

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type Repository interface {
	FindByPrefix(ctx context.Context, prefix string) (*APIKey, error)
	Create(ctx context.Context, key *APIKey) (uuid.UUID, error)
	UpdateLastUsed(ctx context.Context, id uuid.UUID, lastUsed time.Time) error
}
