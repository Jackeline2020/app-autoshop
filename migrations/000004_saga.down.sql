DROP TABLE IF EXISTS processed_messages;
DROP TABLE IF EXISTS outbox;
DROP TABLE IF EXISTS saga_instances;
ALTER TABLE orders DROP COLUMN IF EXISTS cancellation_reason;
ALTER TABLE orders DROP COLUMN IF EXISTS checkout_url;
ALTER TABLE orders DROP COLUMN IF EXISTS waiting_approval_at;
ALTER TABLE orders DROP COLUMN IF EXISTS diagnosis_at;
ALTER TABLE orders DROP COLUMN IF EXISTS diagnosis;
