-- FinTalent testing module. PostgreSQL 14+.
CREATE TABLE IF NOT EXISTS tests (
    id BIGSERIAL PRIMARY KEY,
    author_id BIGINT NOT NULL REFERENCES users(id),
    slug VARCHAR(180) NOT NULL UNIQUE,
    category VARCHAR(120) NOT NULL DEFAULT '',
    difficulty VARCHAR(20) NOT NULL DEFAULT 'medium' CHECK (difficulty IN ('easy','medium','hard')),
    status VARCHAR(20) NOT NULL DEFAULT 'draft' CHECK (status IN ('draft','published','archived','deleted','blocked')),
    visibility VARCHAR(20) NOT NULL DEFAULT 'private' CHECK (visibility IN ('private','public','marketplace')),
    price NUMERIC(12,2) NOT NULL DEFAULT 0 CHECK (price >= 0),
    currency CHAR(3) NOT NULL DEFAULT 'RUB',
    is_free BOOLEAN NOT NULL DEFAULT TRUE,
    current_version INTEGER NOT NULL DEFAULT 1,
    passing_percent NUMERIC(5,2) NOT NULL DEFAULT 60 CHECK (passing_percent BETWEEN 0 AND 100),
    time_limit_seconds INTEGER CHECK (time_limit_seconds IS NULL OR time_limit_seconds > 0),
    moderation_status VARCHAR(20) NOT NULL DEFAULT 'not_required',
    blocked_reason TEXT NOT NULL DEFAULT '',
    blocked_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS test_versions (
    id BIGSERIAL PRIMARY KEY,
    test_id BIGINT NOT NULL REFERENCES tests(id) ON DELETE CASCADE,
    version INTEGER NOT NULL,
    title VARCHAR(240) NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    changelog TEXT NOT NULL DEFAULT '',
    created_by BIGINT NOT NULL REFERENCES users(id),
    published_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(test_id, version)
);

CREATE TABLE IF NOT EXISTS test_questions (
    id BIGSERIAL PRIMARY KEY,
    test_version_id BIGINT NOT NULL REFERENCES test_versions(id) ON DELETE CASCADE,
    sort_order INTEGER NOT NULL DEFAULT 0,
    question TEXT NOT NULL,
    question_type VARCHAR(30) NOT NULL CHECK (question_type IN ('single_choice','multiple_choice','boolean','text','file_upload','matching','ordering','code','case')),
    explanation TEXT NOT NULL DEFAULT '',
    points NUMERIC(8,2) NOT NULL DEFAULT 1 CHECK (points >= 0),
    settings JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS test_answers (
    id BIGSERIAL PRIMARY KEY,
    question_id BIGINT NOT NULL REFERENCES test_questions(id) ON DELETE CASCADE,
    answer TEXT NOT NULL,
    is_correct BOOLEAN NOT NULL DEFAULT FALSE,
    sort_order INTEGER NOT NULL DEFAULT 0,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS test_attempts (
    id BIGSERIAL PRIMARY KEY,
    test_id BIGINT NOT NULL REFERENCES tests(id),
    test_version_id BIGINT NOT NULL REFERENCES test_versions(id),
    user_id BIGINT NOT NULL REFERENCES users(id),
    score NUMERIC(10,2) NOT NULL DEFAULT 0,
    max_score NUMERIC(10,2) NOT NULL DEFAULT 0,
    percent NUMERIC(5,2) NOT NULL DEFAULT 0,
    passed BOOLEAN,
    started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    finished_at TIMESTAMPTZ,
    duration_seconds INTEGER NOT NULL DEFAULT 0,
    status VARCHAR(20) NOT NULL DEFAULT 'started' CHECK (status IN ('started','finished','cancelled')),
    context JSONB NOT NULL DEFAULT '{}'::jsonb
);

CREATE TABLE IF NOT EXISTS test_attempt_answers (
    id BIGSERIAL PRIMARY KEY,
    attempt_id BIGINT NOT NULL REFERENCES test_attempts(id) ON DELETE CASCADE,
    question_id BIGINT NOT NULL REFERENCES test_questions(id),
    selected_answer_id BIGINT REFERENCES test_answers(id),
    text_answer TEXT,
    is_correct BOOLEAN,
    earned_points NUMERIC(8,2) NOT NULL DEFAULT 0,
    answered_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    response_seconds INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE NULLS NOT DISTINCT (attempt_id, question_id, selected_answer_id)
);

CREATE TABLE IF NOT EXISTS test_reviews (
    id BIGSERIAL PRIMARY KEY,
    test_id BIGINT NOT NULL REFERENCES tests(id) ON DELETE CASCADE,
    employer_id BIGINT NOT NULL REFERENCES users(id),
    rating SMALLINT NOT NULL CHECK (rating BETWEEN 1 AND 5),
    comment TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(test_id, employer_id)
);

CREATE TABLE IF NOT EXISTS test_statistics (
    test_id BIGINT PRIMARY KEY REFERENCES tests(id) ON DELETE CASCADE,
    attempts_count BIGINT NOT NULL DEFAULT 0,
    completed_count BIGINT NOT NULL DEFAULT 0,
    passed_count BIGINT NOT NULL DEFAULT 0,
    failed_count BIGINT NOT NULL DEFAULT 0,
    average_percent NUMERIC(5,2) NOT NULL DEFAULT 0,
    average_duration_seconds NUMERIC(12,2) NOT NULL DEFAULT 0,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Future marketplace entitlements and vacancy attachments.
CREATE TABLE IF NOT EXISTS test_entitlements (
    id BIGSERIAL PRIMARY KEY,
    test_id BIGINT NOT NULL REFERENCES tests(id),
    user_id BIGINT NOT NULL REFERENCES users(id),
    source VARCHAR(30) NOT NULL DEFAULT 'purchase',
    price_paid NUMERIC(12,2) NOT NULL DEFAULT 0,
    granted_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ,
    UNIQUE(test_id, user_id, source)
);

CREATE TABLE IF NOT EXISTS vacancy_tests (
    id BIGSERIAL PRIMARY KEY,
    vacancy_external_id BIGINT NOT NULL,
    test_id BIGINT NOT NULL REFERENCES tests(id),
    test_version_id BIGINT REFERENCES test_versions(id),
    sort_order INTEGER NOT NULL DEFAULT 0,
    is_required BOOLEAN NOT NULL DEFAULT TRUE,
    passing_percent NUMERIC(5,2),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(vacancy_external_id, test_id)
);

CREATE INDEX IF NOT EXISTS tests_author_idx ON tests(author_id);
CREATE INDEX IF NOT EXISTS tests_catalog_idx ON tests(status, visibility, category);
CREATE INDEX IF NOT EXISTS test_versions_test_idx ON test_versions(test_id, version DESC);
CREATE INDEX IF NOT EXISTS test_questions_version_idx ON test_questions(test_version_id, sort_order);
CREATE INDEX IF NOT EXISTS test_answers_question_idx ON test_answers(question_id, sort_order);
CREATE INDEX IF NOT EXISTS test_attempts_user_idx ON test_attempts(user_id, started_at DESC);
CREATE INDEX IF NOT EXISTS test_attempts_test_idx ON test_attempts(test_id, started_at DESC);
CREATE INDEX IF NOT EXISTS test_attempt_answers_attempt_idx ON test_attempt_answers(attempt_id);
