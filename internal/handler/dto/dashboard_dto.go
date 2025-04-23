package dto

import (
	"time"

	"github.com/makkenzo/license-service-api/internal/domain/license"
)

type DashboardSummaryResponse struct {
	TotalLicenses int64                           `json:"totalLicenses"`
	StatusCounts  map[license.LicenseStatus]int64 `json:"statusCounts"`
	TypeCounts    map[string]int64                `json:"typeCounts"`
	ExpiringSoon  ExpiringSoonSummary             `json:"expiringSoon"`
	ProductCounts map[string]int64                `json:"productCounts"`
}

type ExpiringSoonSummary struct {
	Count        int64        `json:"count"`
	PeriodDays   int          `json:"periodDays"`
	NextToExpire *LicenseInfo `json:"nextToExpire,omitempty"`
}

type LicenseInfo struct {
	LicenseKey  string    `json:"licenseKey"`
	ExpiresAt   time.Time `json:"expiresAt"`
	ProductName string    `json:"productName"`
}
