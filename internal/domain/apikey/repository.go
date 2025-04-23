package apikey

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
)

var ErrAPIKeyNotFound = errors.New("api key not found or disabled")

type Repository interface {
	FindByPrefix(ctx context.Context, prefix string) (*APIKey, error)
	Create(ctx context.Context, key *APIKey) (uuid.UUID, error)
	UpdateLastUsed(ctx context.Context, id uuid.UUID, lastUsed time.Time) error
}
