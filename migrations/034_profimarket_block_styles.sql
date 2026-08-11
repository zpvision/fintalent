ALTER TABLE profimarket_solutions
    ADD COLUMN IF NOT EXISTS metric_style VARCHAR(40) NOT NULL DEFAULT 'metrics-default',
    ADD COLUMN IF NOT EXISTS access_style VARCHAR(40) NOT NULL DEFAULT 'access-default';

UPDATE profimarket_solutions SET metric_style='metrics-default' WHERE metric_style='';
UPDATE profimarket_solutions SET access_style='access-default' WHERE access_style='';
