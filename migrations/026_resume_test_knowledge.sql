CREATE TABLE IF NOT EXISTS resume_test_confirmations (
    id BIGSERIAL PRIMARY KEY,
    resume_id BIGINT NOT NULL REFERENCES resumes(id) ON DELETE CASCADE,
    test_id BIGINT NOT NULL REFERENCES tests(id) ON DELETE CASCADE,
    confirmer_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (resume_id, test_id, confirmer_id)
);

CREATE INDEX IF NOT EXISTS resume_test_confirmations_resume_idx
    ON resume_test_confirmations(resume_id, test_id, created_at);

-- Demonstration results are added only to resumes other than #1. Existing real
-- attempts are never replaced. The best available published tests are used so
-- this seed remains valid when the test catalogue changes.
WITH candidates AS (
    SELECT r.id AS resume_id, r.user_id, t.id AS test_id, v.id AS version_id,
           ROW_NUMBER() OVER (PARTITION BY r.id ORDER BY t.id) AS test_no
    FROM resumes r
    CROSS JOIN LATERAL (
        SELECT id FROM tests WHERE status='published' ORDER BY id LIMIT 3
    ) t
    JOIN test_versions v ON v.test_id=t.id
       AND v.version=(SELECT current_version FROM tests WHERE id=t.id)
    WHERE r.id<>1 AND r.status='published' AND r.deleted_at IS NULL
), inserted AS (
    INSERT INTO test_attempts(test_id,test_version_id,user_id,score,max_score,percent,passed,started_at,finished_at,duration_seconds,status,context)
    SELECT c.test_id,c.version_id,c.user_id,
           CASE c.test_no WHEN 1 THEN 94 WHEN 2 THEN 88 ELSE 82 END,100,
           CASE c.test_no WHEN 1 THEN 94 WHEN 2 THEN 88 ELSE 82 END,TRUE,
           NOW()-(c.test_no||' days')::interval,
           NOW()-(c.test_no||' days')::interval+((12+c.test_no*3)||' minutes')::interval,
           (12+c.test_no*3)*60,'finished','{"demo_resume_result":true}'::jsonb
    FROM candidates c
    WHERE NOT EXISTS (
        SELECT 1 FROM test_attempts a
        WHERE a.user_id=c.user_id AND a.test_id=c.test_id AND a.status='finished'
    )
    RETURNING id
)
SELECT COUNT(*) FROM inserted;

INSERT INTO resume_test_confirmations(resume_id,test_id,confirmer_id,created_at)
SELECT r.id,a.test_id,u.id,NOW()-((u.id%12)||' hours')::interval
FROM resumes r
JOIN LATERAL (
    SELECT DISTINCT ON (test_id) test_id
    FROM test_attempts
    WHERE user_id=r.user_id AND status='finished'
    ORDER BY test_id,percent DESC,finished_at DESC
) a ON TRUE
JOIN LATERAL (
    SELECT id FROM users WHERE id<>r.user_id ORDER BY id LIMIT 3
) u ON TRUE
WHERE r.id<>1 AND r.status='published' AND r.deleted_at IS NULL
ON CONFLICT DO NOTHING;
