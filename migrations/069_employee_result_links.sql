ALTER TABLE company_test_invitations ADD COLUMN IF NOT EXISTS result_token_hash VARCHAR(64);
ALTER TABLE company_test_invitations ADD COLUMN IF NOT EXISTS result_token_expires_at TIMESTAMPTZ;
CREATE UNIQUE INDEX IF NOT EXISTS company_test_invitations_result_token_hash_idx
    ON company_test_invitations(result_token_hash) WHERE result_token_hash IS NOT NULL;
