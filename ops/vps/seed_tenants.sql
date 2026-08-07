SET app.authenticating = 'true';

INSERT INTO tenants (id, legal_name, code) VALUES
    ('00000000-0000-0000-0000-000000000001', 'Fazal Din Pharma Plus', 'FAZAL_DIN'),
    ('eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee', 'Fazal Din Sandbox', 'SANDBOX')
ON CONFLICT (id) DO NOTHING;

INSERT INTO branches (id, tenant_id, name, code) VALUES
    ('00000000-0000-0000-0000-000000000002', '00000000-0000-0000-0000-000000000001', 'Main Branch', 'MAIN'),
    ('ffffffff-ffff-ffff-ffff-ffffffffffff', 'eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee', 'Sandbox Branch', 'SANDBOX')
ON CONFLICT (tenant_id, id) DO NOTHING;

INSERT INTO counters (id, tenant_id, branch_id, name, code) VALUES
    ('00000000-0000-0000-0000-000000000003', '00000000-0000-0000-0000-000000000001', '00000000-0000-0000-0000-000000000002', 'POS Counter 1', 'POS1')
ON CONFLICT (tenant_id, id) DO NOTHING;
