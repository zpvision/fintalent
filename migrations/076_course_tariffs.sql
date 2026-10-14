ALTER TABLE courses ADD COLUMN IF NOT EXISTS pricing_mode TEXT NOT NULL DEFAULT 'single' CHECK (pricing_mode IN ('single','tariffs'));
CREATE TABLE IF NOT EXISTS course_tariffs (
 id BIGSERIAL PRIMARY KEY,
 course_id BIGINT NOT NULL REFERENCES courses(id) ON DELETE CASCADE,
 title VARCHAR(80) NOT NULL DEFAULT '',
 description VARCHAR(600) NOT NULL DEFAULT '',
 price BIGINT NOT NULL CHECK (price BETWEEN 0 AND 10000000000),
 recommended BOOLEAN NOT NULL DEFAULT FALSE,
 sort_order INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS course_tariffs_order ON course_tariffs(course_id,sort_order,id);
CREATE UNIQUE INDEX IF NOT EXISTS course_tariffs_recommended ON course_tariffs(course_id) WHERE recommended;
