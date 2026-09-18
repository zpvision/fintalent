CREATE TABLE IF NOT EXISTS profimarket_onec_configurations (
    id BIGSERIAL PRIMARY KEY,
    code VARCHAR(80) NOT NULL UNIQUE,
    name VARCHAR(160) NOT NULL UNIQUE,
    logo TEXT NOT NULL DEFAULT '',
    sort_order INTEGER NOT NULL DEFAULT 0,
    active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

INSERT INTO profimarket_onec_configurations(code,name,sort_order) VALUES
('accounting','1С:Бухгалтерия предприятия',10),
('zup','1С:ЗУП',20),
('unf','1С:УНФ',30),
('ut','1С:УТ',40),
('erp','1С:ERP',50),
('other','Другое',100)
ON CONFLICT(code) DO NOTHING;
