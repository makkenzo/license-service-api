package license

import (
	"context"

	"github.com/google/uuid"
)

type Repository interface {
	Create(ctx context.Context, license *License) (uuid.UUID, error)
	FindByID(ctx context.Context, id uuid.UUID) (*License, error)
	FindByKey(ctx context.Context, key string) (*License, error)
	List(ctx context.Context) ([]*License, error)
	Update(ctx context.Context, license *License) error
}
