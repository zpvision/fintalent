ALTER TABLE users
    ADD COLUMN IF NOT EXISTS is_system BOOLEAN NOT NULL DEFAULT FALSE;

WITH system_author AS (
    INSERT INTO users(full_name,email,password_hash,agreed_to_terms,is_blocked,is_system)
    VALUES ('FinTalent','system@fintalent.local',repeat('*',60),TRUE,TRUE,TRUE)
    ON CONFLICT(email) DO UPDATE
       SET full_name='FinTalent',is_blocked=TRUE,is_system=TRUE
    RETURNING id
)
UPDATE tests
SET author_id=(SELECT id FROM system_author)
WHERE (slug LIKE 'position-skill-%'
   OR slug IN (
      'accounting-topic-vat','accounting-topic-profit-tax','accounting-topic-payroll',
      'accounting-topic-accounting-basics','accounting-topic-usn','accounting-topic-fixed-assets',
      'accounting-topic-inventory','accounting-topic-cash','accounting-topic-reporting',
      'accounting-topic-financial-analysis','accounting-topic-one-c','accounting-topic-audit',
      'accounting-topic-management-accounting'
   )) AND author_id IS DISTINCT FROM (SELECT id FROM system_author);

UPDATE test_versions v
SET created_by=u.id
FROM users u, tests t
WHERE u.email='system@fintalent.local'
  AND t.id=v.test_id
  AND (t.slug LIKE 'position-skill-%' OR t.slug IN (
      'accounting-topic-vat','accounting-topic-profit-tax','accounting-topic-payroll',
      'accounting-topic-accounting-basics','accounting-topic-usn','accounting-topic-fixed-assets',
      'accounting-topic-inventory','accounting-topic-cash','accounting-topic-reporting',
      'accounting-topic-financial-analysis','accounting-topic-one-c','accounting-topic-audit',
      'accounting-topic-management-accounting'
  )) AND v.created_by IS DISTINCT FROM u.id;
