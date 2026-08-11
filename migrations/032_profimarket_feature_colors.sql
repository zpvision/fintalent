ALTER TABLE profimarket_access_features
    ADD COLUMN IF NOT EXISTS text_color VARCHAR(20) NOT NULL DEFAULT '#11183c',
    ADD COLUMN IF NOT EXISTS background_color VARCHAR(20) NOT NULL DEFAULT '#ffffff';
