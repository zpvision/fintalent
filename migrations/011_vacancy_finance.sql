ALTER TABLE vacancies
	ADD COLUMN IF NOT EXISTS salary_tax_mode VARCHAR(30) NOT NULL DEFAULT 'net',
	ADD COLUMN IF NOT EXISTS address VARCHAR(500) NOT NULL DEFAULT '';

UPDATE vacancies
SET salary_tax_mode = 'net'
WHERE salary_tax_mode NOT IN ('net', 'gross');
