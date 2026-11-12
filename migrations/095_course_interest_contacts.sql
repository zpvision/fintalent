-- Application contact snapshots; never rewrite historical applications.
ALTER TABLE course_interests ADD COLUMN IF NOT EXISTS student_name TEXT NOT NULL DEFAULT '';
ALTER TABLE course_interests ADD COLUMN IF NOT EXISTS contact_email TEXT NOT NULL DEFAULT '';
ALTER TABLE course_interests ADD COLUMN IF NOT EXISTS contact_phone TEXT NOT NULL DEFAULT '';
ALTER TABLE course_interests ADD COLUMN IF NOT EXISTS tariff_name TEXT NOT NULL DEFAULT '';
CREATE INDEX IF NOT EXISTS course_interests_admin_date ON course_interests(first_clicked_at DESC,id DESC) WHERE user_id IS NOT NULL;
