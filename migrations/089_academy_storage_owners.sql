-- Preserve legacy keys. New keys carry their original storage owner.
ALTER TABLE academy_media_assets ALTER COLUMN storage_key TYPE TEXT;
ALTER TABLE academy_upload_sessions ALTER COLUMN storage_key TYPE TEXT;
ALTER TABLE academy_media_assets ADD COLUMN IF NOT EXISTS storage_owner_id BIGINT REFERENCES users(id);
ALTER TABLE academy_report_jobs ADD COLUMN IF NOT EXISTS storage_owner_id BIGINT REFERENCES users(id);
UPDATE academy_media_assets SET storage_owner_id=created_by WHERE storage_owner_id IS NULL;
UPDATE academy_report_jobs SET storage_owner_id=created_by WHERE storage_owner_id IS NULL AND created_by IS NOT NULL;
CREATE OR REPLACE FUNCTION academy_preserve_storage_owner() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF TG_OP='INSERT' THEN NEW.storage_owner_id:=NEW.created_by;
 ELSIF OLD.storage_owner_id IS NOT NULL AND NEW.storage_owner_id IS DISTINCT FROM OLD.storage_owner_id THEN
 RAISE EXCEPTION 'storage owner is immutable';
 END IF;
 RETURN NEW;
END $$;
DROP TRIGGER IF EXISTS academy_media_storage_owner ON academy_media_assets;
CREATE TRIGGER academy_media_storage_owner BEFORE INSERT OR UPDATE ON academy_media_assets FOR EACH ROW EXECUTE FUNCTION academy_preserve_storage_owner();
DROP TRIGGER IF EXISTS academy_report_storage_owner ON academy_report_jobs;
CREATE TRIGGER academy_report_storage_owner BEFORE INSERT OR UPDATE ON academy_report_jobs FOR EACH ROW EXECUTE FUNCTION academy_preserve_storage_owner();
CREATE INDEX IF NOT EXISTS academy_media_storage_usage ON academy_media_assets(storage_owner_id,storage_driver) WHERE deleted_at IS NULL;
