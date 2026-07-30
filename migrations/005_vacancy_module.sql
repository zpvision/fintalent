ALTER TABLE dictionary_items ADD COLUMN IF NOT EXISTS default_weight INTEGER NOT NULL DEFAULT 5;
ALTER TABLE dictionary_items ADD COLUMN IF NOT EXISTS active BOOLEAN NOT NULL DEFAULT TRUE;
ALTER TABLE dictionary_items ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;

ALTER TABLE vacancy_survey_blocks ADD COLUMN IF NOT EXISTS default_importance VARCHAR(20) NOT NULL DEFAULT 'preferred';

CREATE TABLE IF NOT EXISTS resumes (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    status VARCHAR(20) NOT NULL DEFAULT 'draft' CHECK (status IN ('draft','published','archived')),
    visibility VARCHAR(20) NOT NULL DEFAULT 'private' CHECK (visibility IN ('private','public')),
    current_step INTEGER NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    UNIQUE(user_id)
);

CREATE TABLE IF NOT EXISTS resume_categories (
    id BIGSERIAL PRIMARY KEY,
    resume_id BIGINT NOT NULL REFERENCES resumes(id) ON DELETE CASCADE,
    category_id BIGINT NOT NULL REFERENCES dictionary_items(id) ON DELETE RESTRICT,
    block_id BIGINT REFERENCES applicant_survey_blocks(id) ON DELETE SET NULL,
    sort_order INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(resume_id,category_id)
);

CREATE TABLE IF NOT EXISTS vacancies (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title VARCHAR(240) NOT NULL DEFAULT '',
    description TEXT NOT NULL DEFAULT '',
    status VARCHAR(20) NOT NULL DEFAULT 'draft' CHECK (status IN ('draft','published','archived','closed')),
    salary_from NUMERIC(14,2),
    salary_to NUMERIC(14,2),
    currency CHAR(3) NOT NULL DEFAULT 'RUB',
    employment_type VARCHAR(50) NOT NULL DEFAULT '',
    work_format VARCHAR(50) NOT NULL DEFAULT '',
    city VARCHAR(200) NOT NULL DEFAULT '',
    experience_from INTEGER,
    experience_to INTEGER,
    current_step INTEGER NOT NULL DEFAULT 1,
    requirements_hash CHAR(64) NOT NULL DEFAULT '',
    published_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS vacancy_categories (
    id BIGSERIAL PRIMARY KEY,
    vacancy_id BIGINT NOT NULL REFERENCES vacancies(id) ON DELETE CASCADE,
    category_id BIGINT NOT NULL REFERENCES dictionary_items(id) ON DELETE RESTRICT,
    block_id BIGINT REFERENCES vacancy_survey_blocks(id) ON DELETE SET NULL,
    importance VARCHAR(20) NOT NULL CHECK (importance IN ('required','preferred','bonus')),
    base_weight_snapshot INTEGER NOT NULL,
    importance_coefficient_snapshot INTEGER NOT NULL,
    effective_weight INTEGER NOT NULL,
    sort_order INTEGER NOT NULL DEFAULT 0,
    category_name_snapshot VARCHAR(500) NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(vacancy_id,category_id)
);

CREATE INDEX IF NOT EXISTS resume_categories_category_idx ON resume_categories(category_id,resume_id);
CREATE INDEX IF NOT EXISTS resume_categories_resume_idx ON resume_categories(resume_id,category_id);
CREATE INDEX IF NOT EXISTS resumes_matching_idx ON resumes(status,visibility) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS vacancy_categories_vacancy_idx ON vacancy_categories(vacancy_id,sort_order);
CREATE INDEX IF NOT EXISTS vacancy_categories_category_idx ON vacancy_categories(category_id,vacancy_id);
CREATE INDEX IF NOT EXISTS vacancies_owner_idx ON vacancies(user_id,status) WHERE deleted_at IS NULL;
