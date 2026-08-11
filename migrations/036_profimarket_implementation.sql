CREATE TABLE IF NOT EXISTS profimarket_purchase_button_options (
    id BIGSERIAL PRIMARY KEY,
    code VARCHAR(60) NOT NULL UNIQUE,
    name VARCHAR(120) NOT NULL,
    active BOOLEAN NOT NULL DEFAULT TRUE,
    sort_order INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

INSERT INTO profimarket_purchase_button_options(code,name,sort_order) VALUES
    ('buy','Купить',10),
    ('buy_and_implement','Купить и внедрить',20)
ON CONFLICT(code) DO UPDATE SET name=EXCLUDED.name,sort_order=EXCLUDED.sort_order;

ALTER TABLE profimarket_solutions
    ADD COLUMN IF NOT EXISTS implementation_title VARCHAR(240) NOT NULL DEFAULT 'Регламенты навсегда в вашей CRM',
    ADD COLUMN IF NOT EXISTS implementation_subtitle VARCHAR(500) NOT NULL DEFAULT 'Доступ получаете вы, они остаются у вас',
    ADD COLUMN IF NOT EXISTS purchase_button_code VARCHAR(60) NOT NULL DEFAULT 'buy_and_implement';

