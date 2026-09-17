CREATE TABLE IF NOT EXISTS resume_test_confirmations (
    id BIGSERIAL PRIMARY KEY,
    resume_id BIGINT NOT NULL REFERENCES resumes(id) ON DELETE CASCADE,
    test_id BIGINT NOT NULL REFERENCES tests(id) ON DELETE CASCADE,
    confirmer_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (resume_id, test_id, confirmer_id)
);

CREATE INDEX IF NOT EXISTS resume_test_confirmations_resume_idx
    ON resume_test_confirmations(resume_id, test_id, created_at);
