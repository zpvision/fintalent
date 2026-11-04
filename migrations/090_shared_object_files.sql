CREATE TABLE IF NOT EXISTS object_files (
 id BIGSERIAL PRIMARY KEY,
 legacy_ref TEXT NOT NULL UNIQUE,
 owner_kind TEXT NOT NULL CHECK(owner_kind IN('user','accounting_company','platform')),
 owner_id BIGINT NOT NULL CHECK(owner_id>=0),
 category TEXT NOT NULL CHECK(category IN('academy','tests','files')),
 object_key TEXT NOT NULL UNIQUE,
 storage_id TEXT NOT NULL DEFAULT 'primary' CHECK(storage_id='primary'),
 original_name TEXT NOT NULL, mime TEXT NOT NULL, created_by BIGINT REFERENCES users(id),
 size BIGINT NOT NULL CHECK(size>=0), sha256 TEXT NOT NULL DEFAULT '',
 status TEXT NOT NULL DEFAULT 'pending' CHECK(status IN('pending','ready','failed')),
 created_at TIMESTAMPTZ NOT NULL DEFAULT now(), verified_at TIMESTAMPTZ, lease_until TIMESTAMPTZ,
 CHECK((owner_kind='platform' AND owner_id=0) OR (owner_kind<>'platform' AND owner_id>0))
);
ALTER TABLE object_files ADD COLUMN IF NOT EXISTS lease_until TIMESTAMPTZ;
ALTER TABLE object_files ADD COLUMN IF NOT EXISTS created_by BIGINT REFERENCES users(id);
ALTER TABLE object_files ADD COLUMN IF NOT EXISTS storage_id TEXT NOT NULL DEFAULT 'primary' CHECK(storage_id='primary');
CREATE OR REPLACE FUNCTION object_files_preserve_identity() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF ROW(NEW.legacy_ref,NEW.owner_kind,NEW.owner_id,NEW.category,NEW.object_key,NEW.created_by,NEW.storage_id)
 IS DISTINCT FROM ROW(OLD.legacy_ref,OLD.owner_kind,OLD.owner_id,OLD.category,OLD.object_key,OLD.created_by,OLD.storage_id)
 THEN RAISE EXCEPTION 'file storage identity is immutable'; END IF;
 RETURN NEW;
END $$;
DROP TRIGGER IF EXISTS object_files_identity ON object_files;
CREATE TRIGGER object_files_identity BEFORE UPDATE ON object_files FOR EACH ROW EXECUTE FUNCTION object_files_preserve_identity();
CREATE INDEX IF NOT EXISTS object_files_usage ON object_files(owner_kind,owner_id,category) WHERE status='ready';
CREATE OR REPLACE VIEW file_storage_usage AS
 SELECT owner_kind,owner_id,category,SUM(bytes)::bigint AS bytes FROM (
 SELECT owner_kind,owner_id,category,size AS bytes FROM object_files WHERE status='ready'
 UNION ALL
 SELECT 'user',storage_owner_id,'academy',size FROM academy_media_assets WHERE storage_driver='s3' AND deleted_at IS NULL AND storage_owner_id IS NOT NULL
 UNION ALL
 SELECT CASE WHEN storage_owner_id IS NULL THEN 'platform' ELSE 'user' END,COALESCE(storage_owner_id,0),'academy',size_bytes
 FROM academy_report_jobs WHERE storage_driver='s3' AND status='ready' AND deleted_at IS NULL AND expires_at>now()
 ) sources GROUP BY owner_kind,owner_id,category;
