ALTER TABLE profimarket_solutions DROP CONSTRAINT IF EXISTS profimarket_solutions_type_check;
ALTER TABLE profimarket_solutions ADD CONSTRAINT profimarket_solutions_type_check CHECK(type IN ('REGULATION','AI_ASSISTANT','AUTOMATION','INSTRUCTION','ONEC_INTEGRATION','TEMPLATE','CHECKLIST'));
ALTER TABLE profimarket_solutions ADD COLUMN IF NOT EXISTS product_data JSONB NOT NULL DEFAULT '{}'::jsonb;
CREATE INDEX IF NOT EXISTS profimarket_solutions_type_catalog_idx ON profimarket_solutions(type,status,published_at DESC) WHERE deleted_at IS NULL;

COMMENT ON COLUMN profimarket_solutions.product_data IS 'Расширяемые данные конкретного типа цифрового продукта. Будущие файлы хранятся в delivery_files внутри JSONB.';
