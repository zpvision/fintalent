ALTER TABLE profimarket_regulation_sections
    ADD COLUMN IF NOT EXISTS icon_image_url TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS numbering_color VARCHAR(20) NOT NULL DEFAULT '';

