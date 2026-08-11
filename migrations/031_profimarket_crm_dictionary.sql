ALTER TABLE profimarket_crm
    ADD COLUMN IF NOT EXISTS source_dictionary_item_id BIGINT REFERENCES dictionary_items(id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS description TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS icon TEXT NOT NULL DEFAULT '';

CREATE UNIQUE INDEX IF NOT EXISTS profimarket_crm_source_dictionary_item_uidx
    ON profimarket_crm(source_dictionary_item_id)
    WHERE source_dictionary_item_id IS NOT NULL;
