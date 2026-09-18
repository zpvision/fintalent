ALTER TABLE profimarket_purchases
    ADD COLUMN IF NOT EXISTS seller_seen_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS profimarket_purchases_seller_seen_idx
    ON profimarket_purchases(seller_user_id, seller_seen_at, created_at DESC);
