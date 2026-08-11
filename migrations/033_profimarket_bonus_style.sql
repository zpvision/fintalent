ALTER TABLE profimarket_solutions
    ADD COLUMN IF NOT EXISTS bonus_style VARCHAR(40) NOT NULL DEFAULT 'amber';

UPDATE profimarket_solutions SET bonus_style='amber' WHERE bonus_style='';
