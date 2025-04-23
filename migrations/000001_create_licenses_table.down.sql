DROP TRIGGER IF EXISTS set_timestamp ON licenses;
DROP FUNCTION IF EXISTS trigger_set_timestamp();
DROP INDEX IF EXISTS idx_licenses_customer_email;
DROP INDEX IF EXISTS idx_licenses_expires_at;
DROP INDEX IF EXISTS idx_licenses_status;
DROP TABLE IF EXISTS licenses;
DROP TYPE IF EXISTS license_status;