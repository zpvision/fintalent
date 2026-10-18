-- Independent curriculum. Marketing tables are deliberately never referenced by FK.
CREATE TABLE IF NOT EXISTS academy_course_programs (
 id BIGSERIAL PRIMARY KEY, course_id BIGINT NOT NULL UNIQUE REFERENCES courses(id),
 revision BIGINT NOT NULL DEFAULT 1, source TEXT NOT NULL CHECK(source IN ('empty','landing')),
 created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE TABLE IF NOT EXISTS academy_program_versions (
 id BIGSERIAL PRIMARY KEY, program_id BIGINT NOT NULL REFERENCES academy_course_programs(id),
 number INTEGER NOT NULL, source_revision BIGINT NOT NULL, content JSONB NOT NULL,
 created_by BIGINT NOT NULL REFERENCES users(id), created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
 UNIQUE(program_id,number), UNIQUE(program_id,source_revision)
);
CREATE TABLE IF NOT EXISTS academy_modules (
 id BIGSERIAL PRIMARY KEY, program_id BIGINT NOT NULL REFERENCES academy_course_programs(id),
 key TEXT NOT NULL, title VARCHAR(180) NOT NULL DEFAULT '', description VARCHAR(1200) NOT NULL DEFAULT '',
 cover_asset_id BIGINT, status TEXT NOT NULL DEFAULT 'draft' CHECK(status IN ('draft','published')),
 sort_order INTEGER NOT NULL DEFAULT 0, UNIQUE(program_id,key), UNIQUE(id,program_id)
);
CREATE TABLE IF NOT EXISTS academy_lessons (
 id BIGSERIAL PRIMARY KEY, program_id BIGINT NOT NULL REFERENCES academy_course_programs(id),
 module_id BIGINT NOT NULL, key TEXT NOT NULL, title VARCHAR(180) NOT NULL DEFAULT '',
 description VARCHAR(1200) NOT NULL DEFAULT '', duration_minutes INTEGER NOT NULL DEFAULT 0 CHECK(duration_minutes BETWEEN 0 AND 100000),
 status TEXT NOT NULL DEFAULT 'draft' CHECK(status IN ('draft','published')), cover_asset_id BIGINT,
 required BOOLEAN NOT NULL DEFAULT TRUE, free_preview BOOLEAN NOT NULL DEFAULT FALSE, self_paced BOOLEAN NOT NULL DEFAULT TRUE,
 completion_rule TEXT NOT NULL DEFAULT 'manual' CHECK(completion_rule IN ('manual','materials','test','assignment')),
 sort_order INTEGER NOT NULL DEFAULT 0, UNIQUE(program_id,key), UNIQUE(id,program_id),
 FOREIGN KEY(module_id,program_id) REFERENCES academy_modules(id,program_id)
);
CREATE TABLE IF NOT EXISTS academy_lesson_blocks (
 id BIGSERIAL PRIMARY KEY, program_id BIGINT NOT NULL REFERENCES academy_course_programs(id), lesson_id BIGINT NOT NULL,
 key TEXT NOT NULL, type TEXT NOT NULL CHECK(type IN ('text','video','image','file','link','note','test','assignment')),
 data JSONB NOT NULL DEFAULT '{}', sort_order INTEGER NOT NULL DEFAULT 0, UNIQUE(program_id,key),
 FOREIGN KEY(lesson_id,program_id) REFERENCES academy_lessons(id,program_id)
);
CREATE TABLE IF NOT EXISTS academy_media_assets (
 id BIGSERIAL PRIMARY KEY, program_id BIGINT NOT NULL REFERENCES academy_course_programs(id),
 entity_key TEXT NOT NULL, storage_key VARCHAR(64) NOT NULL UNIQUE, name VARCHAR(240) NOT NULL,
 mime VARCHAR(120) NOT NULL, size BIGINT NOT NULL CHECK(size>0), created_by BIGINT NOT NULL REFERENCES users(id),
 created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), deleted_at TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS academy_modules_order ON academy_modules(program_id,sort_order,id);
CREATE INDEX IF NOT EXISTS academy_lessons_order ON academy_lessons(module_id,sort_order,id);
CREATE INDEX IF NOT EXISTS academy_blocks_order ON academy_lesson_blocks(lesson_id,sort_order,id);
CREATE INDEX IF NOT EXISTS academy_assets_entity ON academy_media_assets(program_id,entity_key);
CREATE TABLE IF NOT EXISTS academy_asset_links (
 asset_id BIGINT NOT NULL REFERENCES academy_media_assets(id), program_id BIGINT NOT NULL REFERENCES academy_course_programs(id),
 entity_key TEXT NOT NULL, PRIMARY KEY(asset_id,entity_key)
);
-- Versions are append-only snapshots. Stable keys inside snapshots identify lessons across editions.
CREATE OR REPLACE FUNCTION academy_immutable_version() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN RAISE EXCEPTION 'Academy versions are immutable'; END $$;
DO $$ BEGIN
 IF NOT EXISTS (SELECT 1 FROM pg_trigger WHERE tgname='academy_version_immutable' AND tgrelid='academy_program_versions'::regclass) THEN
  CREATE TRIGGER academy_version_immutable BEFORE UPDATE OR DELETE ON academy_program_versions FOR EACH ROW EXECUTE FUNCTION academy_immutable_version();
 END IF;
END $$;
