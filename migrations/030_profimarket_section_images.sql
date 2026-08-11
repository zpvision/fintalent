ALTER TABLE profimarket_regulation_sections
    ADD COLUMN IF NOT EXISTS image_url TEXT NOT NULL DEFAULT '';
