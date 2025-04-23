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
