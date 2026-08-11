CREATE TABLE IF NOT EXISTS profimarket_crm (
    id BIGSERIAL PRIMARY KEY,
    code VARCHAR(80) NOT NULL UNIQUE,
    name VARCHAR(160) NOT NULL,
    active BOOLEAN NOT NULL DEFAULT TRUE,
    sort_order INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS profimarket_platforms (
    id BIGSERIAL PRIMARY KEY,
    code VARCHAR(80) NOT NULL UNIQUE,
    name VARCHAR(160) NOT NULL,
    active BOOLEAN NOT NULL DEFAULT TRUE,
    sort_order INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS profimarket_solutions (
    id BIGSERIAL PRIMARY KEY,
    author_user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    type VARCHAR(30) NOT NULL CHECK(type IN ('REGULATION','AI_ASSISTANT')),
    status VARCHAR(30) NOT NULL DEFAULT 'DRAFT' CHECK(status IN ('DRAFT','MODERATION','PUBLISHED','ARCHIVED')),
    title VARCHAR(240) NOT NULL DEFAULT '',
    slug VARCHAR(260) NOT NULL UNIQUE,
    short_description TEXT NOT NULL DEFAULT '',
    description TEXT NOT NULL DEFAULT '',
    cover_image TEXT NOT NULL DEFAULT '',
    price NUMERIC(14,2) NOT NULL DEFAULT 0 CHECK(price>=0),
    old_price NUMERIC(14,2) CHECK(old_price IS NULL OR old_price>=0),
    currency CHAR(3) NOT NULL DEFAULT 'RUB',
    pricing_type VARCHAR(30) NOT NULL DEFAULT 'ONE_TIME' CHECK(pricing_type IN ('ONE_TIME','MONTHLY','YEARLY','FREE')),
    trial_days INTEGER NOT NULL DEFAULT 0 CHECK(trial_days>=0),
    delivery_type VARCHAR(30) NOT NULL DEFAULT 'MANUAL' CHECK(delivery_type IN ('LINK','MANUAL')),
    external_url TEXT NOT NULL DEFAULT '',
    tags TEXT[] NOT NULL DEFAULT '{}',
    topics TEXT[] NOT NULL DEFAULT '{}',
    audiences TEXT[] NOT NULL DEFAULT '{}',
    is_featured BOOLEAN NOT NULL DEFAULT FALSE,
    is_new BOOLEAN NOT NULL DEFAULT TRUE,
    views_count INTEGER NOT NULL DEFAULT 0,
    published_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS profimarket_solutions_catalog_idx ON profimarket_solutions(status,published_at DESC) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS profimarket_solutions_author_idx ON profimarket_solutions(author_user_id,status) WHERE deleted_at IS NULL;

CREATE TABLE IF NOT EXISTS profimarket_media (
    id BIGSERIAL PRIMARY KEY,
    solution_id BIGINT NOT NULL REFERENCES profimarket_solutions(id) ON DELETE CASCADE,
    type VARCHAR(20) NOT NULL CHECK(type IN ('IMAGE','VIDEO')),
    url TEXT NOT NULL,
    sort_order INTEGER NOT NULL DEFAULT 0,
    is_preview BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS profimarket_regulation_sections (
    id BIGSERIAL PRIMARY KEY,
    solution_id BIGINT NOT NULL REFERENCES profimarket_solutions(id) ON DELETE CASCADE,
    title VARCHAR(240) NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    sort_order INTEGER NOT NULL DEFAULT 0
);
CREATE TABLE IF NOT EXISTS profimarket_regulation_items (
    id BIGSERIAL PRIMARY KEY,
    section_id BIGINT NOT NULL REFERENCES profimarket_regulation_sections(id) ON DELETE CASCADE,
    title VARCHAR(300) NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    sort_order INTEGER NOT NULL DEFAULT 0
);
CREATE TABLE IF NOT EXISTS profimarket_access_features (
    id BIGSERIAL PRIMARY KEY,
    solution_id BIGINT NOT NULL REFERENCES profimarket_solutions(id) ON DELETE CASCADE,
    icon VARCHAR(60) NOT NULL DEFAULT 'check',
    title VARCHAR(240) NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    sort_order INTEGER NOT NULL DEFAULT 0
);
CREATE TABLE IF NOT EXISTS profimarket_ai_features (
    id BIGSERIAL PRIMARY KEY,
    solution_id BIGINT NOT NULL REFERENCES profimarket_solutions(id) ON DELETE CASCADE,
    icon VARCHAR(60) NOT NULL DEFAULT 'sparkles',
    title VARCHAR(240) NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    sort_order INTEGER NOT NULL DEFAULT 0
);
CREATE TABLE IF NOT EXISTS profimarket_solution_crm (
    solution_id BIGINT NOT NULL REFERENCES profimarket_solutions(id) ON DELETE CASCADE,
    crm_id BIGINT NOT NULL REFERENCES profimarket_crm(id) ON DELETE RESTRICT,
    PRIMARY KEY(solution_id,crm_id)
);
CREATE TABLE IF NOT EXISTS profimarket_solution_platforms (
    solution_id BIGINT NOT NULL REFERENCES profimarket_solutions(id) ON DELETE CASCADE,
    platform_id BIGINT NOT NULL REFERENCES profimarket_platforms(id) ON DELETE RESTRICT,
    PRIMARY KEY(solution_id,platform_id)
);
CREATE TABLE IF NOT EXISTS profimarket_favorites (
    solution_id BIGINT NOT NULL REFERENCES profimarket_solutions(id) ON DELETE CASCADE,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY(solution_id,user_id)
);
CREATE TABLE IF NOT EXISTS profimarket_purchases (
    id BIGSERIAL PRIMARY KEY,
    solution_id BIGINT NOT NULL REFERENCES profimarket_solutions(id) ON DELETE RESTRICT,
    buyer_user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    seller_user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    amount NUMERIC(14,2) NOT NULL DEFAULT 0,
    currency CHAR(3) NOT NULL DEFAULT 'RUB',
    pricing_type VARCHAR(30) NOT NULL,
    status VARCHAR(30) NOT NULL DEFAULT 'COMPLETED' CHECK(status IN ('PENDING','COMPLETED','CANCELLED','REFUNDED')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE TABLE IF NOT EXISTS profimarket_reviews (
    id BIGSERIAL PRIMARY KEY,
    solution_id BIGINT NOT NULL REFERENCES profimarket_solutions(id) ON DELETE CASCADE,
    purchase_id BIGINT REFERENCES profimarket_purchases(id) ON DELETE SET NULL,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    rating SMALLINT NOT NULL CHECK(rating BETWEEN 1 AND 5),
    comment TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(solution_id,user_id)
);
CREATE TABLE IF NOT EXISTS profimarket_implementation_requests (
    id BIGSERIAL PRIMARY KEY,
    purchase_id BIGINT NOT NULL UNIQUE REFERENCES profimarket_purchases(id) ON DELETE CASCADE,
    solution_id BIGINT NOT NULL REFERENCES profimarket_solutions(id) ON DELETE RESTRICT,
    buyer_user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    seller_user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    crm_id BIGINT REFERENCES profimarket_crm(id) ON DELETE RESTRICT,
    custom_crm_name VARCHAR(200) NOT NULL DEFAULT '',
    crm_email VARCHAR(254) NOT NULL,
    comment TEXT NOT NULL DEFAULT '',
    status VARCHAR(30) NOT NULL DEFAULT 'NEW' CHECK(status IN ('NEW','CONTACTED','IN_PROGRESS','COMPLETED','CANCELLED')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

INSERT INTO profimarket_crm(code,name,sort_order) VALUES
('finkoper','FinKoper',10),('other','Другая CRM',100)
ON CONFLICT(code) DO UPDATE SET name=EXCLUDED.name,active=TRUE,sort_order=EXCLUDED.sort_order;
INSERT INTO profimarket_platforms(code,name,sort_order) VALUES
('telegram','Telegram',10),('max','MAX',20),('web','Web',30),('finkoper','FinKoper',40),('other','Другое',100)
ON CONFLICT(code) DO UPDATE SET name=EXCLUDED.name,active=TRUE,sort_order=EXCLUDED.sort_order;
