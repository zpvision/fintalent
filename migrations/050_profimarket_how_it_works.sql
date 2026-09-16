ALTER TABLE profimarket_solutions ADD COLUMN IF NOT EXISTS how_it_works JSONB NOT NULL DEFAULT '[]'::jsonb;
