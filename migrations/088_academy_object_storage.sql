-- Existing keys and immutable curriculum snapshots remain unchanged.
ALTER TABLE academy_media_assets ADD COLUMN IF NOT EXISTS storage_driver TEXT NOT NULL DEFAULT 'local';
ALTER TABLE academy_media_assets ADD COLUMN IF NOT EXISTS checksum TEXT NOT NULL DEFAULT '';
ALTER TABLE academy_media_assets ADD COLUMN IF NOT EXISTS checksum_kind TEXT NOT NULL DEFAULT '';
ALTER TABLE academy_media_assets ADD COLUMN IF NOT EXISTS storage_updated_at TIMESTAMPTZ NOT NULL DEFAULT now();
ALTER TABLE academy_report_jobs ADD COLUMN IF NOT EXISTS storage_driver TEXT NOT NULL DEFAULT 'local';
CREATE TABLE IF NOT EXISTS academy_upload_sessions (
 id TEXT PRIMARY KEY, program_id BIGINT NOT NULL REFERENCES academy_course_programs(id),
 course_id BIGINT NOT NULL REFERENCES courses(id), user_id BIGINT NOT NULL REFERENCES users(id),
 entity_key TEXT NOT NULL, storage_key VARCHAR(64) NOT NULL UNIQUE, backend_id TEXT NOT NULL,
 name VARCHAR(240) NOT NULL, mime VARCHAR(120) NOT NULL, size BIGINT NOT NULL CHECK(size>0),
 part_size BIGINT NOT NULL CHECK(part_size>=5242880), upload_id TEXT NOT NULL DEFAULT '',
 status TEXT NOT NULL DEFAULT 'initiating' CHECK(status IN('initiating','uploading','finalizing','ready','cancelled','failed')),
 asset_id BIGINT REFERENCES academy_media_assets(id), created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 expires_at TIMESTAMPTZ NOT NULL DEFAULT now()+INTERVAL '24 hours',
 checksum_offset BIGINT NOT NULL DEFAULT 0, checksum_state TEXT NOT NULL DEFAULT '', object_etag TEXT NOT NULL DEFAULT '',
 lease_until TIMESTAMPTZ, updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
ALTER TABLE academy_upload_sessions ADD COLUMN IF NOT EXISTS checksum_offset BIGINT NOT NULL DEFAULT 0;
ALTER TABLE academy_upload_sessions ADD COLUMN IF NOT EXISTS checksum_state TEXT NOT NULL DEFAULT '';
ALTER TABLE academy_upload_sessions ADD COLUMN IF NOT EXISTS object_etag TEXT NOT NULL DEFAULT '';
ALTER TABLE academy_upload_sessions ADD COLUMN IF NOT EXISTS error_code TEXT NOT NULL DEFAULT '';
CREATE INDEX IF NOT EXISTS academy_upload_cleanup ON academy_upload_sessions(status,expires_at);
CREATE INDEX IF NOT EXISTS academy_upload_owner ON academy_upload_sessions(user_id,created_at);
CREATE TABLE IF NOT EXISTS academy_storage_gc (
 storage_key TEXT NOT NULL, storage_driver TEXT NOT NULL, attempts INT NOT NULL DEFAULT 0,
 next_at TIMESTAMPTZ NOT NULL DEFAULT now()+INTERVAL '24 hours', lease_until TIMESTAMPTZ,
 PRIMARY KEY(storage_key,storage_driver)
);
