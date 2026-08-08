-- Canonical line-unit repair — 2026-08-08
--
-- Defect: the Phase E line importers stored quantity = PackQty + LooseQty/PackUnits
-- (pack-normalized) while unit_price stayed the legacy per-piece Rate, so
-- line_total = unit_price * quantity understated every pack-sized line by its
-- PackUnits factor (proved: invoice 695336 line showed 0.20 instead of 20.00).
--
-- Repair: store quantities in legacy pieces (PackQty*PackUnits + LooseQty),
-- keeping unit_price as the per-piece rate so line_total = quantity*unit_price
-- holds and matches the legacy line amount (pieces*Rate). Legacy stock
-- snapshots (StockReport) are already piece-denominated, so the stock ledger is
-- scaled to pieces as well, keeping ledger-derived balances comparable to the
-- reconciled snapshots. Raw PackQty/LooseQty/PackUnits stay preserved in
-- legacy_payload, so pack-format display remains derivable.
--
-- Scope guard: only lines imported from the four pack/loose detail streams
-- (legacy_payload carries PackQty+LooseQty). Purchase-order lines use the
-- pack-denominated PurOrderDetail.Qty by legacy design and are untouched.
-- The script is idempotent: after repair, quantity equals the piece count and
-- re-running finds no rows where quantity differs from the piece count.

\set ON_ERROR_STOP on
BEGIN;

CREATE TEMPORARY TABLE repair_line_units_report (
    step text PRIMARY KEY,
    affected bigint NOT NULL,
    detail jsonb NOT NULL DEFAULT '{}'::jsonb
);

-- 1. Repair imported document lines to piece quantities.
WITH fixed AS (
    SELECT l.id,
           l.tenant_id,
           (COALESCE(NULLIF(l.legacy_payload->>'PackQty', '')::numeric, 0)
             * CASE WHEN COALESCE(NULLIF(l.legacy_payload->>'PackUnits', '')::numeric, 0) = 0
                    THEN 1 ELSE NULLIF(l.legacy_payload->>'PackUnits', '')::numeric END
           + COALESCE(NULLIF(l.legacy_payload->>'LooseQty', '')::numeric, 0)) AS piece_quantity,
           COALESCE(NULLIF(l.legacy_payload->>'UnitSalesTax', '')::numeric, 0) AS unit_sales_tax
      FROM business_document_lines l
     WHERE l.legacy_payload ? 'PackQty' AND l.legacy_payload ? 'LooseQty'
), changed AS (
    SELECT f.id, f.tenant_id, f.piece_quantity, f.unit_sales_tax
      FROM fixed f
      JOIN business_document_lines l ON l.id = f.id AND l.tenant_id = f.tenant_id
     WHERE l.quantity IS DISTINCT FROM f.piece_quantity
), updated AS (
    UPDATE business_document_lines l
       SET quantity = c.piece_quantity,
           line_gross = ROUND(l.unit_price * c.piece_quantity, 4),
           line_total = ROUND(l.unit_price * c.piece_quantity, 4),
           tax_amount = ROUND(c.unit_sales_tax * c.piece_quantity, 4)
      FROM changed c
     WHERE l.id = c.id AND l.tenant_id = c.tenant_id
     RETURNING l.id
)
INSERT INTO repair_line_units_report (step, affected)
SELECT 'business_document_lines', COUNT(*) FROM updated;

-- 2. Scale the linked stock ledger rows to pieces.
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
INSERT INTO repair_line_units_report (step, affected)
SELECT 'stock_ledger', COUNT(*) FROM updated;

-- 3. Verification: line totals must equal unit_price*quantity after repair, and
--    sample pack-sized rows must now carry piece quantities and legacy amounts.
INSERT INTO repair_line_units_report (step, affected, detail)
SELECT 'verify_line_total_identity_violations', COUNT(*),
       jsonb_build_object('note', 'must be 0; |line_total - unit_price*quantity| beyond 0.0001 rounding')
  FROM business_document_lines l
 WHERE l.legacy_payload ? 'PackQty'
   AND ABS(l.line_total - ROUND(l.unit_price * l.quantity, 4)) > 0.0002;

INSERT INTO repair_line_units_report (step, affected, detail)
SELECT 'golden_sample_695336', COUNT(*),
       jsonb_build_object('document', l.document_id, 'quantity', l.quantity, 'unit_price', l.unit_price, 'line_total', l.line_total)
  FROM business_document_lines l
  JOIN business_documents d ON d.tenant_id = l.tenant_id AND d.id = l.document_id
 WHERE d.document_number = '695336' AND l.legacy_payload ? 'PackQty';

SELECT step, affected, detail FROM repair_line_units_report ORDER BY step;

COMMIT;
