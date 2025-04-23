CREATE TYPE license_status AS ENUM (
    'pending',
    'active',
    'inactive',
    'expired',
    'revoked'
);

CREATE TABLE IF NOT EXISTS licenses (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    license_key   TEXT UNIQUE NOT NULL,
    status        license_status NOT NULL DEFAULT 'pending',
    type          VARCHAR(50) NOT NULL,
    customer_name VARCHAR(255),
    customer_email VARCHAR(255),
    product_name  VARCHAR(100) NOT NULL,
    metadata      JSONB,
    issued_at     TIMESTAMPTZ,
    expires_at    TIMESTAMPTZ,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

COMMENT ON COLUMN licenses.id IS 'Unique identifier for the license record (UUID)';
COMMENT ON COLUMN licenses.license_key IS 'The actual license key provided to the customer';
COMMENT ON COLUMN licenses.status IS 'Current status of the license (pending, active, inactive, expired, revoked)';
COMMENT ON COLUMN licenses.type IS 'Type or tier of the license (e.g., trial, basic, pro)';
COMMENT ON COLUMN licenses.customer_name IS 'Name of the customer or company';
COMMENT ON COLUMN licenses.customer_email IS 'Email address of the customer';
COMMENT ON COLUMN licenses.product_name IS 'Name of the product this license is for';
COMMENT ON COLUMN licenses.metadata IS 'Flexible JSONB field for additional license data (limits, features, activation info, etc.)';
COMMENT ON COLUMN licenses.issued_at IS 'Timestamp when the license was officially issued or activated';
COMMENT ON COLUMN licenses.expires_at IS 'Timestamp when the license expires (NULL for perpetual)';
COMMENT ON COLUMN licenses.created_at IS 'Timestamp when the license record was created in the database';
COMMENT ON COLUMN licenses.updated_at IS 'Timestamp when the license record was last updated';

CREATE INDEX IF NOT EXISTS idx_licenses_status ON licenses (status);
CREATE INDEX IF NOT EXISTS idx_licenses_expires_at ON licenses (expires_at);
CREATE INDEX IF NOT EXISTS idx_licenses_customer_email ON licenses (customer_email);

CREATE OR REPLACE FUNCTION trigger_set_timestamp()
RETURNS TRIGGER AS $$
BEGIN
  IF row(NEW.*) IS DISTINCT FROM row(OLD.*) THEN
    NEW.updated_at = NOW();
    RETURN NEW;
  ELSE
    RETURN OLD;
  END IF;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER set_timestamp
BEFORE UPDATE ON licenses
FOR EACH ROW
EXECUTE FUNCTION trigger_set_timestamp();