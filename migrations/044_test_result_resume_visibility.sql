ALTER TABLE test_attempts
    ADD COLUMN IF NOT EXISTS show_in_resume BOOLEAN;

UPDATE test_attempts
SET show_in_resume = (status = 'finished' AND percent > 80)
WHERE show_in_resume IS NULL;

ALTER TABLE test_attempts
    ALTER COLUMN show_in_resume SET DEFAULT FALSE,
    ALTER COLUMN show_in_resume SET NOT NULL;

CREATE INDEX IF NOT EXISTS test_attempts_resume_visibility_idx
    ON test_attempts(user_id, show_in_resume, test_id, percent DESC)
    WHERE status = 'finished';
