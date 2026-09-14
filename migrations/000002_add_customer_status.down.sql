DROP INDEX IF EXISTS idx_customers_status;
ALTER TABLE customers DROP COLUMN IF EXISTS status;
