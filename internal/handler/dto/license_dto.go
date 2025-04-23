package dto

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/makkenzo/license-service-api/internal/domain/license"
)

type CreateLicenseRequest struct {
	Type          string                 `json:"type" binding:"required"`
	ProductName   string                 `json:"product_name" binding:"required"`
	CustomerName  *string                `json:"customer_name"`
	CustomerEmail *string                `json:"customer_email" binding:"omitempty,email"`
	Metadata      json.RawMessage        `json:"metadata" swaggertype:"object"`
	ExpiresAt     *time.Time             `json:"expires_at" binding:"omitempty,gt"`
	InitialStatus *license.LicenseStatus `json:"initial_status,omitempty"`
}

type LicenseResponse struct {
	ID            uuid.UUID             `json:"id"`
	LicenseKey    string                `json:"license_key"`
	Status        license.LicenseStatus `json:"status"`
	Type          string                `json:"type"`
	CustomerName  *string               `json:"customer_name,omitempty"`
	CustomerEmail *string               `json:"customer_email,omitempty"`
	ProductName   string                `json:"product_name"`
	Metadata      json.RawMessage       `json:"metadata,omitempty" swaggertype:"object"`
	IssuedAt      *time.Time            `json:"issued_at,omitempty"`
	ExpiresAt     *time.Time            `json:"expires_at,omitempty"`
	CreatedAt     time.Time             `json:"created_at"`
	UpdatedAt     time.Time             `json:"updated_at"`
}

func NewLicenseResponse(lic *license.License) *LicenseResponse {
	resp := &LicenseResponse{
		ID:          lic.ID,
		LicenseKey:  lic.LicenseKey,
		Status:      lic.Status,
		Type:        lic.Type,
		ProductName: lic.ProductName,
		Metadata:    lic.Metadata,
		CreatedAt:   lic.CreatedAt,
		UpdatedAt:   lic.UpdatedAt,
	}
	if lic.CustomerName.Valid {
		resp.CustomerName = &lic.CustomerName.String
	}
	if lic.CustomerEmail.Valid {
		resp.CustomerEmail = &lic.CustomerEmail.String
	}
	if lic.IssuedAt.Valid {
		resp.IssuedAt = &lic.IssuedAt.Time
	}
	if lic.ExpiresAt.Valid {
		resp.ExpiresAt = &lic.ExpiresAt.Time
	}
	return resp
}

type ListLicensesRequest struct {
	Status        *license.LicenseStatus `form:"status" binding:"omitempty,oneof=pending active inactive expired revoked"`
	CustomerEmail *string                `form:"email" binding:"omitempty,email"`
	ProductName   *string                `form:"product_name"`
	Type          *string                `form:"type"`
	Limit         int                    `form:"limit,default=20" binding:"omitempty,gte=0"`
	Offset        int                    `form:"offset,default=0" binding:"omitempty,gte=0"`
	SortBy        string                 `form:"sort_by,default=created_at"`
	SortOrder     string                 `form:"sort_order,default=DESC" binding:"omitempty,oneof=ASC DESC"`
}

type PaginatedLicenseResponse struct {
	Licenses   []*LicenseResponse `json:"licenses"`
	TotalCount int64              `json:"totalCount"`
	Limit      int                `json:"limit"`
	Offset     int                `json:"offset"`
}

type UpdateLicenseRequest struct {
	Type          *string         `json:"type"`
	CustomerName  *string         `json:"customer_name"`
	CustomerEmail *string         `json:"customer_email" binding:"omitempty,email"`
	ProductName   *string         `json:"product_name"`
	Metadata      json.RawMessage `json:"metadata" swaggertype:"object"`
	ExpiresAt     *time.Time      `json:"expires_at" binding:"omitempty,gt"`
}

type UpdateLicenseStatusRequest struct {
	Status *license.LicenseStatus `json:"status" binding:"required,oneof=pending active inactive expired revoked"`
}

type ValidateLicenseRequest struct {
	LicenseKey  string          `json:"license_key" binding:"required"`
	ProductName string          `json:"product_name" binding:"required"`
	Metadata    json.RawMessage `json:"metadata,omitempty" swaggertype:"object"`
}

type ValidateLicenseResponse struct {
	IsValid bool `json:"is_valid"`

	Status      *license.LicenseStatus `json:"status,omitempty"`
	Reason      string                 `json:"reason,omitempty"`
	ExpiresAt   *time.Time             `json:"expires_at,omitempty"`
	AllowedData json.RawMessage        `json:"allowed_data,omitempty"`
}
