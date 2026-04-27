-- T4 — direct schema change on production payments table.
-- This migration is the canonical "must restructure" example: a synchronous
-- ALTER on a billion-row payments table with no online migration plan, no
-- backfill strategy, no rollback. Hard block — must be split into:
--   1. additive column with default
--   2. backfill via job
--   3. constrain non-null after backfill
-- The reviewer should flag this T4 with a "must restructure" verdict.

BEGIN;

ALTER TABLE payments
  ADD COLUMN settlement_batch_id BIGINT NOT NULL DEFAULT 0;

CREATE INDEX idx_payments_settlement_batch_id ON payments(settlement_batch_id);

COMMIT;
