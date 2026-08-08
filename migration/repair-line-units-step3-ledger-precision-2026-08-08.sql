-- Canonical line-unit repair — step 3 of 3 — 2026-08-08
--
-- Precision correction for step 2 (migration/repair-line-units-step2-ledger-2026-08-08.sql).
--
-- Step 2 scaled the OLD stock_ledger.quantity by PackUnits. That is
-- algebraically equivalent to the correct piece quantity, but stock_ledger's
-- quantity column is numeric(19,4) — the OLD pack-normalized value was
-- already rounded to 4 decimals before step 2 ever touched it, and
-- multiplying an already-lossy value by PackUnits amplifies that rounding
-- (observed: e.g. 9.9990 vs the true 10.0000 — errors up to ~0.002 in a
-- spot sample, worse for larger PackUnits). Root-caused after step 2's own
-- verify_ledger_line_mismatch check reported 124,399 non-zero rows (with a
-- tight 0.0002 tolerance) instead of 0.
--
-- Fix: confirmed via COUNT(*) = COUNT(DISTINCT source_document_line_id)
-- (781,203 = 781,203) that every affected stock_ledger row maps 1:1 to
-- exactly one business_document_lines row — no batch-split allocations
-- exist in this dataset. So the correct value is simply the now-exact,
-- already-corrected business_document_lines.quantity (step 1, committed)
-- directly — no rescaling of the old lossy value, no proportional math,
-- zero precision loss.
--
-- Same scoped-trigger-bypass approach as step 2 (human-approved 2026-08-08),
-- applied here to correct step 2's own rounding artifact, not to make a new
-- architectural decision. stock_ledger_backup_20260808 (pre-step-2 state)
-- remains available for reference/rollback if ever needed.

\set ON_ERROR_STOP on
BEGIN;

CREATE TEMPORARY TABLE repair_stock_ledger_precision_report (
    step text PRIMARY KEY,
    affected bigint NOT NULL,
    detail jsonb NOT NULL DEFAULT '{}'::jsonb
);

ALTER TABLE stock_ledger DISABLE TRIGGER stock_ledger_immutable;

WITH changed AS (
    SELECT sl.id, sl.tenant_id, l.quantity AS exact_quantity
      FROM stock_ledger sl
      JOIN business_document_lines l
        ON l.tenant_id = sl.tenant_id AND l.id = sl.source_document_line_id
     WHERE l.legacy_payload ? 'PackQty' AND l.legacy_payload ? 'LooseQty'
       AND sl.quantity IS DISTINCT FROM l.quantity
), updated AS (
    UPDATE stock_ledger sl
       SET quantity = c.exact_quantity
      FROM changed c
     WHERE sl.id = c.id AND sl.tenant_id = c.tenant_id
     RETURNING sl.id
)
INSERT INTO repair_stock_ledger_precision_report (step, affected)
SELECT 'stock_ledger_precision', COUNT(*) FROM updated;

ALTER TABLE stock_ledger ENABLE TRIGGER stock_ledger_immutable;

INSERT INTO repair_stock_ledger_precision_report (step, affected, detail)
SELECT 'trigger_reenabled', COUNT(*),
       jsonb_build_object('note', 'must be 1 row, tgenabled=O (origin, i.e. enabled)')
  FROM pg_trigger
 WHERE tgrelid = 'stock_ledger'::regclass AND tgname = 'stock_ledger_immutable' AND tgenabled = 'O';

-- Must now be exactly 0 (exact equality, no tolerance needed at all).
INSERT INTO repair_stock_ledger_precision_report (step, affected, detail)
SELECT 'verify_ledger_line_mismatch', COUNT(*),
       jsonb_build_object('note', 'must be 0; stock_ledger.quantity should now equal source business_document_lines.quantity exactly')
  FROM stock_ledger sl
  JOIN business_document_lines l ON l.tenant_id = sl.tenant_id AND l.id = sl.source_document_line_id
 WHERE l.legacy_payload ? 'PackQty'
   AND sl.quantity IS DISTINCT FROM l.quantity;

SELECT step, affected, detail FROM repair_stock_ledger_precision_report ORDER BY step;

COMMIT;
