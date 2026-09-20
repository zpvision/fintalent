CREATE TABLE IF NOT EXISTS profimarket_compatibility_options (
    id BIGSERIAL PRIMARY KEY,
    code VARCHAR(80) NOT NULL UNIQUE,
    name VARCHAR(160) NOT NULL UNIQUE,
    sort_order INTEGER NOT NULL DEFAULT 0,
    active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

INSERT INTO profimarket_compatibility_options(code,name,sort_order) VALUES
('onec','1С',10),
('finkoper','FinKoper',20),
('excel','Excel',30),
('google-sheets','Google Sheets',40),
('telegram','Telegram',50),
('email','Email',60),
('kontur','Контур',70),
('sbis','СБИС',80),
('bitrix24','Битрикс24',90),
('amocrm','amoCRM',100),
('n8n','n8n',110),
('make','Make',120),
('api','API',130),
('other','Другое',140)
ON CONFLICT(code) DO NOTHING;

CREATE TABLE IF NOT EXISTS profimarket_data_updates (
    key TEXT PRIMARY KEY,
    applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM profimarket_data_updates WHERE key='059_automation_compatibility') THEN
        UPDATE profimarket_solutions
        SET product_data=jsonb_set(CASE WHEN jsonb_typeof(product_data)='object' THEN product_data ELSE '{}'::jsonb END,'{compatibility}','["FinKoper","Контур"]'::jsonb,TRUE),
            updated_at=NOW()
        WHERE type='AUTOMATION'
          AND slug='novoe-reshenie-465323137b'
          AND deleted_at IS NULL;

        UPDATE profimarket_solutions
        SET product_data=jsonb_set(
                CASE WHEN jsonb_typeof(product_data)='object' THEN product_data ELSE '{}'::jsonb END,
                '{compatibility}',
                CASE MOD(id,5)
                    WHEN 0 THEN '["1С","Excel"]'::jsonb
                    WHEN 1 THEN '["Telegram","Email"]'::jsonb
                    WHEN 2 THEN '["Битрикс24","amoCRM"]'::jsonb
                    WHEN 3 THEN '["n8n","Make","API"]'::jsonb
                    ELSE '["СБИС","Контур"]'::jsonb
                END,
                TRUE
            ),
            updated_at=NOW()
        WHERE type='AUTOMATION'
          AND slug<>'novoe-reshenie-465323137b'
          AND deleted_at IS NULL
          AND jsonb_array_length(CASE WHEN jsonb_typeof(product_data)='object' AND jsonb_typeof(product_data->'compatibility')='array' THEN product_data->'compatibility' ELSE '[]'::jsonb END)=0;

        INSERT INTO profimarket_data_updates(key) VALUES('059_automation_compatibility');
    END IF;
END $$;
