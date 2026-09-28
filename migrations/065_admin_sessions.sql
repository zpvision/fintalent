CREATE TABLE IF NOT EXISTS admin_sessions (
    token_hash CHAR(64) PRIMARY KEY,
    credential_hash CHAR(64) NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS admin_sessions_expires_at_idx ON admin_sessions(expires_at);
