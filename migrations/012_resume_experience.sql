CREATE TABLE IF NOT EXISTS resume_work_experiences (
    id BIGSERIAL PRIMARY KEY,
    resume_id BIGINT NOT NULL REFERENCES resumes(id) ON DELETE CASCADE,
    company_name VARCHAR(240) NOT NULL,
    position VARCHAR(240) NOT NULL,
    city VARCHAR(200) NOT NULL DEFAULT '',
    industry_item_id BIGINT REFERENCES dictionary_items(id) ON DELETE SET NULL,
    start_month SMALLINT NOT NULL CHECK (start_month BETWEEN 1 AND 12),
    start_year SMALLINT NOT NULL CHECK (start_year BETWEEN 1950 AND 2200),
    end_month SMALLINT CHECK (end_month BETWEEN 1 AND 12),
    end_year SMALLINT CHECK (end_year BETWEEN 1950 AND 2200),
    is_current BOOLEAN NOT NULL DEFAULT FALSE,
    responsibilities TEXT NOT NULL DEFAULT '',
    achievements TEXT NOT NULL DEFAULT '',
    sort_order INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (
        (is_current = TRUE AND end_month IS NULL AND end_year IS NULL)
        OR
        (is_current = FALSE AND end_month IS NOT NULL AND end_year IS NOT NULL)
    )
);

CREATE INDEX IF NOT EXISTS resume_work_experiences_resume_idx
    ON resume_work_experiences(resume_id, sort_order, id);

CREATE TABLE IF NOT EXISTS resume_work_experience_duties (
    experience_id BIGINT NOT NULL REFERENCES resume_work_experiences(id) ON DELETE CASCADE,
    duty_id BIGINT NOT NULL REFERENCES duties(id) ON DELETE RESTRICT,
    sort_order INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (experience_id, duty_id)
);

CREATE INDEX IF NOT EXISTS resume_work_experience_duties_duty_idx
    ON resume_work_experience_duties(duty_id, experience_id);
