BEGIN;
ALTER TABLE payments ADD COLUMN settlement_batch_id BIGINT NOT NULL DEFAULT 0;
CREATE INDEX idx_payments_settlement_batch_id ON payments(settlement_batch_id);
COMMIT;
