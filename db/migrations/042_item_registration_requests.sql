-- Phase F: preserve the captured ItemRegRequest workflow as a tenant-scoped
-- request history. The source row has 130 fields; the source-shaped payload
-- retains every imported Item field while the request lifecycle metadata is
-- promoted to typed columns. This migration does not claim to send requests
-- to the legacy registration server.

CREATE SEQUENCE IF NOT EXISTS master_item_registration_request_code_seq;

CREATE TABLE IF NOT EXISTS master_item_registration_requests (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    item_id uuid NOT NULL REFERENCES master_items(id) ON DELETE CASCADE,
    legacy_item_id text NOT NULL,
    request_code integer NOT NULL,
    requested_at timestamptz NOT NULL DEFAULT now(),
    server_name text NOT NULL DEFAULT '',
    machine_name text NOT NULL DEFAULT '',
    sent char(1) NOT NULL DEFAULT 'N' CHECK (sent IN ('Y', 'N')),
    sent_on timestamptz,
    sent_by integer,
    server_request_code integer,
    payload jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, request_code)
);

CREATE INDEX IF NOT EXISTS idx_master_item_registration_requests_item
    ON master_item_registration_requests (tenant_id, item_id, requested_at DESC);

ALTER TABLE master_item_registration_requests ENABLE ROW LEVEL SECURITY;
ALTER TABLE master_item_registration_requests FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS master_item_registration_requests_tenant_scope ON master_item_registration_requests;
CREATE POLICY master_item_registration_requests_tenant_scope ON master_item_registration_requests
    USING (
        tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid
        OR current_setting('app.authenticating', true) = 'true'
    )
    WITH CHECK (
        tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid
        OR current_setting('app.authenticating', true) = 'true'
    );
