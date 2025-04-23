package license

import (
	"database/sql"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type LicenseStatus string

const (
	StatusPending  LicenseStatus = "pending"
	StatusActive   LicenseStatus = "active"
	StatusInactive LicenseStatus = "inactive"
	StatusExpired  LicenseStatus = "expired"
	StatusRevoked  LicenseStatus = "revoked"
)

type License struct {
	ID            uuid.UUID       `db:"id" json:"id"`
	LicenseKey    string          `db:"license_key" json:"license_key"`
	Status        LicenseStatus   `db:"status" json:"status"`
	Type          string          `db:"type" json:"type"`
	CustomerName  sql.NullString  `db:"customer_name" json:"customer_name,omitempty"`
	CustomerEmail sql.NullString  `db:"customer_email" json:"customer_email,omitempty"`
	ProductName   string          `db:"product_name" json:"product_name"`
	Metadata      json.RawMessage `db:"metadata" json:"metadata,omitempty"`
	IssuedAt      sql.NullTime    `db:"issued_at" json:"issued_at,omitempty"`
	ExpiresAt     sql.NullTime    `db:"expires_at" json:"expires_at,omitempty"`
	CreatedAt     time.Time       `db:"created_at" json:"created_at"`
	UpdatedAt     time.Time       `db:"updated_at" json:"updated_at"`
}

func (l *License) SetMetadata(data interface{}) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}
	l.Metadata = jsonData
	return nil
}

func (l *License) GetMetadata(target interface{}) error {
	if l.Metadata == nil {
		return nil
	}
	return json.Unmarshal(l.Metadata, target)
}
