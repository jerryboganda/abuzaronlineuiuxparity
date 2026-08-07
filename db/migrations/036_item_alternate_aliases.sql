-- Phase F item-master contextual command support.
--
-- AlternateItemAlias is distinct from the primary item alias/barcode fields.
-- Keeping a separate alias kind lets the Item Form command replace alternate
-- names without silently deleting the primary AliasName or barcode lookup.

ALTER TABLE master_aliases
    DROP CONSTRAINT IF EXISTS master_aliases_alias_kind_check;

ALTER TABLE master_aliases
    ADD CONSTRAINT master_aliases_alias_kind_check
    CHECK (alias_kind IN ('alias', 'alternate_alias', 'barcode', 'legacy_id'));

CREATE INDEX IF NOT EXISTS idx_master_aliases_alternate_item
    ON master_aliases (tenant_id, item_id, alias_kind, active, alias_value)
    WHERE alias_kind = 'alternate_alias';
