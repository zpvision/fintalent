CREATE TABLE IF NOT EXISTS app_migrations (
    name VARCHAR(200) PRIMARY KEY,
    applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM app_migrations WHERE name='006_vacancy_per_answer_importance') THEN
        UPDATE vacancy_categories SET importance='required';
        INSERT INTO app_migrations(name) VALUES('006_vacancy_per_answer_importance');
    END IF;
END $$;
ALTER TABLE vacancy_categories ALTER COLUMN importance SET DEFAULT 'required';
ALTER TABLE vacancy_categories ALTER COLUMN base_weight_snapshot SET DEFAULT 0;
ALTER TABLE vacancy_categories ALTER COLUMN importance_coefficient_snapshot SET DEFAULT 0;
ALTER TABLE vacancy_categories ALTER COLUMN effective_weight SET DEFAULT 0;
CREATE INDEX IF NOT EXISTS vacancy_categories_vacancy_category_idx ON vacancy_categories(vacancy_id,category_id);
CREATE INDEX IF NOT EXISTS vacancy_categories_vacancy_block_category_idx ON vacancy_categories(vacancy_id,block_id,category_id);
-- default_weight, default_importance and snapshot columns are retained only for
-- backwards-compatible rollback. The application no longer reads or writes them.
