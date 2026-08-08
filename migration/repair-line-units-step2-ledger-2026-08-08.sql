-- Canonical line-unit repair — step 2 of 2 — 2026-08-08
--
-- Scales stock_ledger.quantity to pieces to stay consistent with the now
-- piece-denominated business_document_lines.quantity (step 1) and with the
-- piece-denominated legacy StockReport snapshots already reconciled.
--
-- stock_ledger is deliberately immutable (see db/migrations/012_stock_ledger.sql,
-- trigger stock_ledger_immutable / reject_stock_ledger_mutation, asserted by
-- TestStockMigrationDefinesImmutableBatchLedgerCacheAndRLS). This is a ONE-TIME
-- migration-tooling data repair correcting a known import defect — not a live
-- business mutation the trigger is meant to prevent. The trigger is disabled
-- for the minimum scope (this transaction only) and re-enabled before COMMIT,
-- so the invariant is intact at every moment except mid-transaction, during
-- which ALTER TABLE's ACCESS EXCLUSIVE lock also blocks any concurrent access.
-- A full backup (stock_ledger_backup_20260808) was taken beforehand.
--
-- Human-approved approach (2026-08-08): "Scoped trigger bypass" — see session
-- record. Alternative considered and rejected for this session: truncate +
-- full reimport (larger blast radius via FKs from stock_allocations /
-- stock_balances / stock_balance_rebuilds, not fully scoped).

\set ON_ERROR_STOP on
BEGIN;

CREATE TEMPORARY TABLE repair_stock_ledger_report (
    step text PRIMARY KEY,
    affected bigint NOT NULL,
    detail jsonb NOT NULL DEFAULT '{}'::jsonb
);

ALTER TABLE stock_ledger DISABLE TRIGGER stock_ledger_immutable;

WITH scaled AS (
    SELECT sl.id,
           sl.tenant_id,
           ROUND(sl.quantity * CASE WHEN COALESCE(NULLIF(l.legacy_payload->>'PackUnits', '')::numeric, 0) = 0
                                    THEN 1 ELSE NULLIF(l.legacy_payload->>'PackUnits', '')::numeric END, 8) AS piece_quantity
      FROM stock_ledger sl
      JOIN business_document_lines l
        ON l.tenant_id = sl.tenant_id AND l.id = sl.source_document_line_id
     WHERE l.legacy_payload ? 'PackQty' AND l.legacy_payload ? 'LooseQty'
), changed AS (
    SELECT s.id, s.tenant_id, s.piece_quantity
      FROM scaled s
      JOIN stock_ledger sl ON sl.id = s.id AND sl.tenant_id = s.tenant_id
     WHERE sl.quantity IS DISTINCT FROM s.piece_quantity
), updated AS (
    UPDATE stock_ledger sl
       SET quantity = c.piece_quantity
      FROM changed c
     WHERE sl.id = c.id AND sl.tenant_id = c.tenant_id
     RETURNING sl.id
)
INSERT INTO repair_stock_ledger_report (step, affected)
SELECT 'stock_ledger', COUNT(*) FROM updated;

ALTER TABLE stock_ledger ENABLE TRIGGER stock_ledger_immutable;

-- Verify the trigger is back on before we trust anything past this point.
INSERT INTO repair_stock_ledger_report (step, affected, detail)
SELECT 'trigger_reenabled', COUNT(*),
       jsonb_build_object('note', 'must be 1 row, tgenabled=O (origin, i.e. enabled)')
  FROM pg_trigger
 WHERE tgrelid = 'stock_ledger'::regclass AND tgname = 'stock_ledger_immutable' AND tgenabled = 'O';

-- Verify the scaling matches business_document_lines.quantity for pack-sized
-- lines that produced a stock movement (allow small rounding tolerance).
INSERT INTO repair_stock_ledger_report (step, affected, detail)
SELECT 'verify_ledger_line_mismatch', COUNT(*),
       jsonb_build_object('note', 'must be 0; stock_ledger.quantity should equal source business_document_lines.quantity within rounding')
  FROM stock_ledger sl
  JOIN business_document_lines l ON l.tenant_id = sl.tenant_id AND l.id = sl.source_document_line_id
 WHERE l.legacy_payload ? 'PackQty'
   AND ABS(sl.quantity - l.quantity) > 0.0002;

SELECT step, affected, detail FROM repair_stock_ledger_report ORDER BY step;

COMMIT;
