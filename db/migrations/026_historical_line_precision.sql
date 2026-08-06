-- Legacy quantities can include fractional loose units whose divisor is not a
-- power of ten. Preserve the imported calculation instead of rounding every
-- line to four decimal places before reconciliation or later stock posting.
ALTER TABLE business_document_lines
    ALTER COLUMN quantity TYPE numeric(19, 8);
