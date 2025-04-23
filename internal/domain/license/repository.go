package license

import (
	"context"
	"errors"
	"time"

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

type DashboardSummaryData struct {
	TotalCount        int64
	StatusCounts      map[LicenseStatus]int64
	TypeCounts        map[string]int64
	ExpiringSoonCount int64
	NextToExpireKey   *string
	NextToExpireDate  *time.Time
	NextToExpireProd  *string
	ProductCounts     map[string]int64
}

type Repository interface {
	Create(ctx context.Context, license *License) (uuid.UUID, error)
	FindByID(ctx context.Context, id uuid.UUID) (*License, error)
	FindByKey(ctx context.Context, key string) (*License, error)
	List(ctx context.Context, params ListParams) ([]*License, int64, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, status LicenseStatus) error
	Update(ctx context.Context, license *License) error
	GetDashboardSummary(ctx context.Context, expiringPeriodDays int) (*DashboardSummaryData, error)
}

var (
	ErrNotFound     = errors.New("license not found")
	ErrDuplicateKey = errors.New("license key already exists")
	ErrUpdateFailed = errors.New("license update failed")
)
