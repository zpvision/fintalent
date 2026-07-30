CREATE TABLE IF NOT EXISTS resume_educations (
    id BIGSERIAL PRIMARY KEY,
    resume_id BIGINT NOT NULL REFERENCES resumes(id) ON DELETE CASCADE,
    education_type VARCHAR(40) NOT NULL CHECK (education_type IN (
        'higher', 'incomplete_higher', 'secondary_vocational',
        'secondary', 'professional_retraining', 'course', 'certificate', 'other'
    )),
    institution VARCHAR(300) NOT NULL,
    specialization VARCHAR(300) NOT NULL DEFAULT '',
    city VARCHAR(200) NOT NULL DEFAULT '',
    start_year SMALLINT NOT NULL CHECK (start_year BETWEEN 1950 AND 2200),
    end_year SMALLINT CHECK (end_year BETWEEN 1950 AND 2200),
    is_current BOOLEAN NOT NULL DEFAULT FALSE,
    description TEXT NOT NULL DEFAULT '',
    sort_order INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (
        (is_current = TRUE AND end_year IS NULL)
        OR
        (is_current = FALSE AND end_year IS NOT NULL)
    )
);

CREATE INDEX IF NOT EXISTS resume_educations_resume_idx
    ON resume_educations(resume_id, sort_order, id);

ALTER TABLE resume_educations ALTER COLUMN start_year DROP NOT NULL;
ALTER TABLE resume_educations DROP CONSTRAINT IF EXISTS resume_educations_start_year_check;

CREATE TABLE IF NOT EXISTS resume_education_certificates (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    storage_name VARCHAR(160) NOT NULL UNIQUE,
    original_name VARCHAR(300) NOT NULL,
    content_type VARCHAR(100) NOT NULL,
    file_size BIGINT NOT NULL CHECK (file_size > 0 AND file_size <= 8388608),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

ALTER TABLE resume_educations
    ADD COLUMN IF NOT EXISTS certificate_id BIGINT REFERENCES resume_education_certificates(id) ON DELETE SET NULL;

CREATE TABLE IF NOT EXISTS resume_search_statuses (
    code VARCHAR(40) PRIMARY KEY,
    name VARCHAR(160) NOT NULL,
    sort_order INTEGER NOT NULL DEFAULT 0,
    is_active BOOLEAN NOT NULL DEFAULT TRUE
);

ALTER TABLE resume_search_statuses
    ADD COLUMN IF NOT EXISTS icon VARCHAR(500) NOT NULL DEFAULT '';

INSERT INTO resume_search_statuses(code,name,sort_order) VALUES
    ('open','Готов(а) к предложениям',0),
    ('considering','Рассматриваю интересные предложения',1),
    ('not_active','Сейчас не ищу, но открыт(а) к общению',2),
    ('hidden','Не показывать работодателям',3)
ON CONFLICT(code) DO UPDATE SET name=EXCLUDED.name,sort_order=EXCLUDED.sort_order;

UPDATE resume_search_statuses
SET icon = CASE code
    WHEN 'open' THEN '/static/icons/resume-status/open.svg'
    WHEN 'considering' THEN '/static/icons/resume-status/considering.svg'
    WHEN 'not_active' THEN '/static/icons/resume-status/not-active.svg'
    WHEN 'hidden' THEN '/static/icons/resume-status/hidden.svg'
    ELSE icon
END;

ALTER TABLE dictionaries
    ADD COLUMN IF NOT EXISTS resume_title VARCHAR(300) NOT NULL DEFAULT '';

ALTER TABLE resumes ADD COLUMN IF NOT EXISTS desired_salary NUMERIC(14,2);
ALTER TABLE resumes ADD COLUMN IF NOT EXISTS available_immediately BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE resumes ADD COLUMN IF NOT EXISTS search_status_code VARCHAR(40) REFERENCES resume_search_statuses(code);
ALTER TABLE resumes ADD COLUMN IF NOT EXISTS published_at TIMESTAMPTZ;

CREATE TABLE IF NOT EXISTS languages (
    id BIGSERIAL PRIMARY KEY,
    code VARCHAR(12) NOT NULL UNIQUE,
    name VARCHAR(100) NOT NULL,
    sort_order INTEGER NOT NULL DEFAULT 0,
    is_active BOOLEAN NOT NULL DEFAULT TRUE
);

INSERT INTO languages(code,name,sort_order) VALUES
    ('ru','Русский',1),
    ('en','Английский',2),
    ('de','Немецкий',3),
    ('fr','Французский',4),
    ('es','Испанский',5),
    ('it','Итальянский',6),
    ('zh','Китайский',7),
    ('ja','Японский',8),
    ('ko','Корейский',9),
    ('pt','Португальский',10),
    ('ar','Арабский',11),
    ('tr','Турецкий',12),
    ('pl','Польский',13),
    ('cs','Чешский',14),
    ('uk','Украинский',15),
    ('be','Белорусский',16),
    ('kk','Казахский',17),
    ('uz','Узбекский',18),
    ('hy','Армянский',19),
    ('he','Иврит',20)
ON CONFLICT(code) DO UPDATE SET
    name = EXCLUDED.name,
    sort_order = EXCLUDED.sort_order,
    is_active = TRUE;

CREATE TABLE IF NOT EXISTS resume_languages (
    resume_id BIGINT NOT NULL REFERENCES resumes(id) ON DELETE CASCADE,
    language_id BIGINT NOT NULL REFERENCES languages(id) ON DELETE RESTRICT,
    sort_order INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY(resume_id, language_id)
);

CREATE INDEX IF NOT EXISTS resume_languages_resume_idx
    ON resume_languages(resume_id, sort_order, language_id);

ALTER TABLE users ADD COLUMN IF NOT EXISTS avatar_url VARCHAR(500) NOT NULL DEFAULT '';

UPDATE users
SET avatar_url = '/static/profile-3-avatar.png'
WHERE LOWER(email) = '3@3.ru' AND avatar_url = '';

ALTER TABLE resumes
    ADD COLUMN IF NOT EXISTS preferred_city_id BIGINT;

CREATE TABLE IF NOT EXISTS resume_work_formats (
    resume_id BIGINT NOT NULL REFERENCES resumes(id) ON DELETE CASCADE,
    dictionary_item_id BIGINT NOT NULL REFERENCES dictionary_items(id) ON DELETE RESTRICT,
    sort_order INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY(resume_id, dictionary_item_id)
);

CREATE INDEX IF NOT EXISTS resume_work_formats_resume_idx
    ON resume_work_formats(resume_id, sort_order, dictionary_item_id);

CREATE TABLE IF NOT EXISTS resume_preferred_cities (
    resume_id BIGINT NOT NULL REFERENCES resumes(id) ON DELETE CASCADE,
    city_id BIGINT NOT NULL,
    sort_order INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY(resume_id, city_id)
);

CREATE INDEX IF NOT EXISTS resume_preferred_cities_resume_idx
    ON resume_preferred_cities(resume_id, sort_order, city_id);

INSERT INTO resume_preferred_cities(resume_id,city_id,sort_order)
SELECT id,preferred_city_id,0
FROM resumes
WHERE preferred_city_id IS NOT NULL
ON CONFLICT(resume_id,city_id) DO NOTHING;

UPDATE dictionary_items i
SET icon = CASE i.value
    WHEN 'Офис' THEN '/static/icons/work-format/office.svg'
    WHEN 'Гибрид' THEN '/static/icons/work-format/hybrid.svg'
    WHEN 'Удаленно' THEN '/static/icons/work-format/remote.svg'
    WHEN 'Удалённо' THEN '/static/icons/work-format/remote.svg'
    ELSE i.icon
END
FROM dictionaries d
WHERE d.id=i.dictionary_id
  AND d.alias='work_format'
  AND i.value IN ('Офис','Гибрид','Удаленно','Удалённо');
