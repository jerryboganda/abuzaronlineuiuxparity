-- Canonical line-unit repair — step 1 of 2 — 2026-08-08
--
-- Fixes business_document_lines only (the money-facing bug: line_total was
-- understated by the PackUnits factor for pack-sized lines). Split from the
-- original combined script so this verified fix commits independently of
-- step 2 (stock_ledger), which requires a separate scoped trigger bypass.
-- See migration/repair-line-units-step2-ledger-2026-08-08.sql for step 2.

\set ON_ERROR_STOP on
BEGIN;

CREATE TEMPORARY TABLE repair_line_units_report (
    step text PRIMARY KEY,
    affected bigint NOT NULL,
    detail jsonb NOT NULL DEFAULT '{}'::jsonb
);

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

INSERT INTO repair_line_units_report (step, affected, detail)
SELECT 'verify_line_total_identity_violations', COUNT(*),
       jsonb_build_object('note', 'must be 0; |line_total - unit_price*quantity| beyond 0.0001 rounding')
  FROM business_document_lines l
 WHERE l.legacy_payload ? 'PackQty'
   AND ABS(l.line_total - ROUND(l.unit_price * l.quantity, 4)) > 0.0002;

INSERT INTO repair_line_units_report (step, affected, detail)
SELECT 'golden_sample_695336', COUNT(*),
       jsonb_build_object('rows', jsonb_agg(jsonb_build_object(
           'document', l.document_id, 'quantity', l.quantity,
           'unit_price', l.unit_price, 'line_total', l.line_total)))
  FROM business_document_lines l
  JOIN business_documents d ON d.tenant_id = l.tenant_id AND d.id = l.document_id
 WHERE d.document_number = '695336' AND l.legacy_payload ? 'PackQty';

SELECT step, affected, detail FROM repair_line_units_report ORDER BY step;

COMMIT;
