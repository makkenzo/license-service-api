package license

import (
	"context"
	"errors"

	"github.com/google/uuid"
)

type ListParams struct {
	Status        *LicenseStatus
	CustomerEmail *string
	ProductName   *string
	Type          *string
	Limit         int
	Offset        int
	SortBy        string
	SortOrder     string
}

type Repository interface {
	Create(ctx context.Context, license *License) (uuid.UUID, error)
	FindByID(ctx context.Context, id uuid.UUID) (*License, error)
	FindByKey(ctx context.Context, key string) (*License, error)
	List(ctx context.Context, params ListParams) ([]*License, int64, error)
	Update(ctx context.Context, license *License) error
}

var (
	ErrNotFound     = errors.New("license not found")
	ErrDuplicateKey = errors.New("license key already exists")
)
