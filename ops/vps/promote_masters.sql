SET app.authenticating = 'true';

INSERT INTO master_items (tenant_id, legacy_id, code, name, payload, active, created_at, updated_at)
SELECT tenant_id, COALESCE(NULLIF(legacy_id, ''), code), code, name, payload, active, created_at, updated_at
FROM master_records WHERE tenant_id = '00000000-0000-0000-0000-000000000001'::uuid AND kind = 'item'
ON CONFLICT (tenant_id, legacy_id) DO UPDATE SET code=EXCLUDED.code, name=EXCLUDED.name, payload=EXCLUDED.payload, active=EXCLUDED.active, updated_at=EXCLUDED.updated_at;

INSERT INTO master_parties (tenant_id, party_type, legacy_id, code, name, payload, active, created_at, updated_at)
SELECT tenant_id, kind, COALESCE(NULLIF(legacy_id, ''), code), code, name, payload, active, created_at, updated_at
FROM master_records WHERE tenant_id = '00000000-0000-0000-0000-000000000001'::uuid AND kind IN ('customer', 'supplier')
ON CONFLICT (tenant_id, party_type, legacy_id) DO UPDATE SET code=EXCLUDED.code, name=EXCLUDED.name, payload=EXCLUDED.payload, active=EXCLUDED.active, updated_at=EXCLUDED.updated_at;

INSERT INTO master_manufacturers (tenant_id, legacy_id, code, name, payload, active, created_at, updated_at)
SELECT tenant_id, COALESCE(NULLIF(legacy_id, ''), code), code, name, payload, active, created_at, updated_at
FROM master_records WHERE tenant_id = '00000000-0000-0000-0000-000000000001'::uuid AND kind = 'manufacturer'
ON CONFLICT (tenant_id, legacy_id) DO UPDATE SET code=EXCLUDED.code, name=EXCLUDED.name, payload=EXCLUDED.payload, active=EXCLUDED.active, updated_at=EXCLUDED.updated_at;

INSERT INTO master_godowns (tenant_id, legacy_id, code, name, payload, active, created_at, updated_at)
SELECT tenant_id, COALESCE(NULLIF(legacy_id, ''), code), code, name, payload, active, created_at, updated_at
FROM master_records WHERE tenant_id = '00000000-0000-0000-0000-000000000001'::uuid AND kind = 'godown'
ON CONFLICT (tenant_id, legacy_id) DO UPDATE SET code=EXCLUDED.code, name=EXCLUDED.name, payload=EXCLUDED.payload, active=EXCLUDED.active, updated_at=EXCLUDED.updated_at;

INSERT INTO master_aliases (tenant_id, item_id, alias_kind, alias_value, normalized_value)
SELECT i.tenant_id, i.id, 'legacy_id', i.legacy_id, lower(i.legacy_id)
FROM master_items i WHERE i.tenant_id = '00000000-0000-0000-0000-000000000001'::uuid
ON CONFLICT (tenant_id, alias_kind, normalized_value) DO NOTHING;
