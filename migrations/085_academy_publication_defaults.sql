-- Defaults affect future inserts only; existing drafts and immutable versions retain their statuses.
ALTER TABLE academy_modules ALTER COLUMN status SET DEFAULT 'published';
ALTER TABLE academy_lessons ALTER COLUMN status SET DEFAULT 'published';
