CREATE TABLE IF NOT EXISTS password_reset_requests (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT REFERENCES users(id) ON DELETE CASCADE,
    email_hash CHAR(64) NOT NULL,
    code_hash CHAR(64) NOT NULL,
    reset_token_hash CHAR(64),
    request_ip VARCHAR(64) NOT NULL,
    failed_attempts INTEGER NOT NULL DEFAULT 0,
    expires_at TIMESTAMPTZ NOT NULL,
    reset_expires_at TIMESTAMPTZ,
    verified_at TIMESTAMPTZ,
    used_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS password_reset_email_created_idx ON password_reset_requests(email_hash, created_at DESC);
CREATE INDEX IF NOT EXISTS password_reset_ip_created_idx ON password_reset_requests(request_ip, created_at DESC);
CREATE UNIQUE INDEX IF NOT EXISTS password_reset_token_idx ON password_reset_requests(reset_token_hash) WHERE reset_token_hash IS NOT NULL;

