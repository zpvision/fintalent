-- Existing links get a full transition window; historical attempts remain intact.
ALTER TABLE company_test_invitations ADD COLUMN IF NOT EXISTS expires_at TIMESTAMPTZ NOT NULL DEFAULT (NOW() + INTERVAL '30 days');
