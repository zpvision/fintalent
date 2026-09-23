CREATE TABLE IF NOT EXISTS profimarket_questions (
    id BIGSERIAL PRIMARY KEY,
    solution_id BIGINT NOT NULL REFERENCES profimarket_solutions(id) ON DELETE CASCADE,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    question TEXT NOT NULL,
    answer TEXT NOT NULL DEFAULT '',
    answered_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_profimarket_questions_solution_created
    ON profimarket_questions(solution_id, created_at DESC, id DESC);
