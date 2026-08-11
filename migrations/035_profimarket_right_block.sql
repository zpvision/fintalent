ALTER TABLE profimarket_solutions
    ADD COLUMN IF NOT EXISTS right_block_title VARCHAR(120) NOT NULL DEFAULT 'Формат и доступ';

UPDATE profimarket_solutions SET right_block_title='Формат и доступ' WHERE right_block_title='';

ALTER TABLE profimarket_solutions DROP COLUMN IF EXISTS layout_config;
