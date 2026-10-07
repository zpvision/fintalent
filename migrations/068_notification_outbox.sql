CREATE TABLE IF NOT EXISTS notification_outbox (
    id BIGSERIAL PRIMARY KEY,
    kind TEXT NOT NULL,
    recipient_name TEXT NOT NULL DEFAULT '',
    recipient_email TEXT NOT NULL,
    subject TEXT NOT NULL DEFAULT '',
    payload JSONB NOT NULL DEFAULT '{}',
    dedup_key TEXT UNIQUE,
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','sending','sent','failed')),
    attempts INTEGER NOT NULL DEFAULT 0,
    next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    sent_at TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS notification_outbox_pending_idx ON notification_outbox(next_attempt_at,id) WHERE status IN ('pending','sending');
ALTER TABLE profimarket_purchases ADD COLUMN IF NOT EXISTS request_key TEXT;
ALTER TABLE profimarket_purchases ADD COLUMN IF NOT EXISTS request_fingerprint TEXT;
CREATE UNIQUE INDEX IF NOT EXISTS profimarket_purchase_request_idx ON profimarket_purchases(buyer_user_id,request_key) WHERE request_key IS NOT NULL;
