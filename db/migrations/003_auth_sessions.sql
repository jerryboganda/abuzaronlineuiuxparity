-- Server-side HTTP-only session records.
-- Token material is never stored; only a SHA-256 token hash is persisted.
-- Sessions are authentication metadata and are intentionally resolved before
-- tenant RLS context is established.

CREATE TABLE IF NOT EXISTS sessions (
    token_hash text PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    branch_id uuid REFERENCES branches(id) ON DELETE CASCADE,
    counter_id uuid REFERENCES counters(id) ON DELETE CASCADE,
    created_at timestamptz NOT NULL DEFAULT now(),
    last_seen_at timestamptz NOT NULL DEFAULT now(),
    expires_at timestamptz NOT NULL,
    FOREIGN KEY (tenant_id, user_id) REFERENCES users(tenant_id, id),
    FOREIGN KEY (tenant_id, branch_id) REFERENCES branches(tenant_id, id),
    FOREIGN KEY (tenant_id, branch_id, counter_id) REFERENCES counters(tenant_id, branch_id, id)
);

CREATE INDEX IF NOT EXISTS idx_sessions_user ON sessions (user_id, expires_at);
CREATE INDEX IF NOT EXISTS idx_sessions_expiry ON sessions (expires_at);
