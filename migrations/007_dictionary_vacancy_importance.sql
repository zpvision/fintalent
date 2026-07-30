ALTER TABLE dictionaries
    ADD COLUMN IF NOT EXISTS use_importance_in_vacancy BOOLEAN NOT NULL DEFAULT TRUE;
UPDATE dictionaries SET use_importance_in_vacancy=TRUE WHERE use_importance_in_vacancy IS NULL;
