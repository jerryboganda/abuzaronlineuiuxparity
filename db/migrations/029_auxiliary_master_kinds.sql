-- Phase F auxiliary Basic Data master kinds.
--
-- These captured leaves use the tenant-scoped master_records compatibility
-- target until a source-backed normalized table is approved. They are still
-- real CRUD records, not client-only placeholders; the payload retains the
-- source-shaped fields and the base columns provide code/name/active lookup.

ALTER TABLE master_records
    DROP CONSTRAINT IF EXISTS master_records_kind_check;

ALTER TABLE master_records
    ADD CONSTRAINT master_records_kind_check CHECK (kind IN (
        'customer', 'supplier', 'item', 'user', 'category', 'manufacturer',
        'template', 'area', 'godown', 'godown_group', 'item_group',
        'customer_group', 'supplier_category', 'manufacturer_category',
        'item_category', 'customer_category', 'price_policy',
        'company_header', 'config_setting', 'preference',
        'sale-promotion', 'customer-sector', 'generic-item',
        'item-basic-data', 'price-policy', 'item-alert',
        'sales-tax-schedule', 'pct-codes', 'generic-item-type',
        'item-thickness', 'lock-reason', 'category-segment',
        'manufacturer-type', 'sale-template', 'tax-category',
        'item-class', 'item-category', 'supplier-category',
        'manufacturer-category', 'item_group', 'items'
    ));
