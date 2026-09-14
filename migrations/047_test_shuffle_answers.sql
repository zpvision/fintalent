-- Keep answer ordering with the version used by attempts and invitations.
-- Nullable only during backfill: subsequent starts must preserve explicit false.
ALTER TABLE test_versions ADD COLUMN IF NOT EXISTS shuffle_answers BOOLEAN;

UPDATE test_versions v
SET shuffle_answers = EXISTS (
    SELECT 1 FROM test_questions q
    WHERE q.test_version_id = v.id
      AND q.settings @> '{"shuffle_answers":true}'::jsonb
)
WHERE v.shuffle_answers IS NULL;

ALTER TABLE test_versions ALTER COLUMN shuffle_answers SET DEFAULT FALSE;
ALTER TABLE test_versions ALTER COLUMN shuffle_answers SET NOT NULL;
